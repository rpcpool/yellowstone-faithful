package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
)

const varintSize = 10

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
