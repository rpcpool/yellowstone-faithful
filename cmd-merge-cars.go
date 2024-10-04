package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
)

const (
	varintSize       = 10
	nulRootCarHeader = "\x19" + // 25 bytes of CBOR (encoded as varint :cryingbear: )
		// map with 2 keys
		"\xA2" +
		// text-key with length 5
		"\x65" + "roots" +
		// 1 element array
		"\x81" +
		// tag 42
		"\xD8\x2A" +
		// bytes with length 5
		"\x45" +
		// nul-identity-cid prefixed with \x00 as required in DAG-CBOR: https://github.com/ipld/specs/blob/master/block-layer/codecs/dag-cbor.md#links
		"\x00\x01\x55\x00\x00" +
		// text-key with length 7
		"\x67" + "version" +
		// 1, we call this v0 due to the nul-identity CID being an open question: https://github.com/ipld/go-car/issues/26#issuecomment-604299576
		"\x01"
)

func newCmd_MergeCars() *cli.Command {
	var outputFile string
	return &cli.Command{
		Name:        "merge-cars",
		Description: "Merges split car files into a single file",
		Usage:       "Merges split car files into a single file",
		ArgsUsage:   "<list of car files>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "output-file",
				Aliases:     []string{"o"},
				Usage:       "Output file name",
				Required:    true,
				Destination: &outputFile,
			},
		},
		Action: func(c *cli.Context) error {
			paths := c.Args().Slice()

			out, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()

			w := bufio.NewWriter(out)

			if _, err := io.WriteString(w, nulRootCarHeader); err != nil {
				return fmt.Errorf("failed to write empty header: %w", err)
			}

			for _, path := range paths {
				f, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open input file %s: %w", path, err)
				}
				defer f.Close()

				r := bufio.NewReader(f)
				err = discardHeader(r)
				if err != nil {
					return fmt.Errorf("failed to discard header: %w", err)
				}

				io.Copy(w, r)

			}

			return nil
		},
	}
}

func discardHeader(streamBuf *bufio.Reader) error {
	maybeHeaderLen, err := streamBuf.Peek(varintSize)
	if err != nil {
		return fmt.Errorf("failed to read header: %s\n", err)
	}

	hdrLen, viLen := binary.Uvarint(maybeHeaderLen)
	if hdrLen <= 0 || viLen < 0 {
		return fmt.Errorf("unexpected header len = %d, varint len = %d\n", hdrLen, viLen)
	}

	_, err = io.CopyN(io.Discard, streamBuf, int64(viLen))
	if err != nil {
		return fmt.Errorf("failed to discard header varint: %s\n", err)
	}

	// ignoring header decoding for now
	_, err = io.CopyN(io.Discard, streamBuf, int64(hdrLen))
	if err != nil {
		return fmt.Errorf("failed to discard header header: %s\n", err)
	}

	return nil
}
