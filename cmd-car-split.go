package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

const (
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

type FileInfo struct {
	FileName  string
	FirstSlot int
	LastSlot  int
}

func newCmd_SplitCar() *cli.Command {
	return &cli.Command{
		Name:        "split-car",
		Description: "Splits an epoch car file into smaller chunks. Each chunk corresponds to a subset.",
		ArgsUsage:   "<epoch-car-path>",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:     "size",
				Aliases:  []string{"s"},
				Value:    31 * 1024 * 1024 * 1024, // 31 GiB
				Usage:    "Target size in bytes to chunk CARs to.",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			carPath := c.Args().First()
			var file fs.File
			var err error
			if carPath == "-" {
				file = os.Stdin
			} else {
				file, err = os.Open(carPath)
				if err != nil {
					klog.Exit(err.Error())
				}
				defer file.Close()
			}

			rd, err := carreader.New(file)
			if err != nil {
				klog.Exitf("Failed to open CAR: %s", err)
			}
			{
				// print roots:
				roots := rd.Header.Roots
				klog.Infof("Roots: %d", len(roots))
				for i, root := range roots {
					if i == 0 && len(roots) == 1 {
						klog.Infof("- %s (Epoch CID)", root.String())
					} else {
						klog.Infof("- %s", root.String())
					}
				}
			}

			maxFileSize := c.Int64("size")

			var (
				currentFileSize int64
				currentFileNum  int
				currentFile     *os.File
				fileMutex       sync.Mutex
				currentFileInfo FileInfo
			)

			createNewFile := func() error {
				fileMutex.Lock()
				defer fileMutex.Unlock()

				if currentFile != nil {
					currentFile.Close()
				}
				currentFileNum++
				filename := fmt.Sprintf("%d.car", currentFileNum)
				currentFile, err = os.Create(filename)
				if err != nil {
					return err
				}
				// Write the header
				_, err = io.WriteString(currentFile, nulRootCarHeader)
				if err != nil {
					return err
				}

				// Set the currentFileSize to the size of the header
				currentFileSize = int64(len(nulRootCarHeader))
				currentFileInfo = FileInfo{FileName: filename, FirstSlot: -1, LastSlot: -1}
				return nil
			}

			writeObject := func(data []byte) error {
				fileMutex.Lock()
				defer fileMutex.Unlock()

				if currentFile == nil || currentFileSize+int64(len(data)) > maxFileSize {
					if err := createNewFile(); err != nil {
						return err
					}
				}

				_, err := currentFile.Write(data)
				if err != nil {
					return fmt.Errorf("failed to write object: %w", err)
				}
				currentFileSize += int64(len(data))
				return nil
			}

			processObject := func(data []byte) error {
				kind, err := iplddecoders.GetKind(data)
				if err != nil {
					return err
				}

				if kind == iplddecoders.KindBlock {
					block, err := iplddecoders.DecodeBlock(data)
					if err != nil {
						return err
					}

					if currentFileInfo.FirstSlot == -1 || block.Slot < currentFileInfo.FirstSlot {
						currentFileInfo.FirstSlot = block.Slot
					}
					if block.Slot > currentFileInfo.LastSlot {
						currentFileInfo.LastSlot = block.Slot
					}
				}

				return writeObject(data)
			}

			accum := accum.NewObjectAccumulator(
				rd,
				iplddecoders.KindBlock,
				func(owm1 *accum.ObjectWithMetadata, owm2 []accum.ObjectWithMetadata) error {
					for _, owm := range owm2 {
						if err := processObject(owm.ObjectData); err != nil {
							return err
						}
					}

					if err := processObject(owm1.ObjectData); err != nil {
						return err
					}
					return nil
				},
				iplddecoders.KindEpoch,
				iplddecoders.KindSubset,
			)

			if err := accum.Run((context.Background())); err != nil {
				klog.Exitf("error while accumulating objects: %w", err)
			}

			// To do: Construct and write the SubSet node

			return nil
		},
	}
}
