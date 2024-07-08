package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multicodec"
	"github.com/rpcpool/yellowstone-faithful/accum"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
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

type subsetInfo struct {
	fileName   string
	firstSlot  int
	lastSlot   int
	blockLinks []datamodel.Link
}

func newCmd_SplitCar() *cli.Command {
	return &cli.Command{
		Name:        "split-car",
		Description: "Splits an epoch car file into smaller chunks. Each chunk corresponds to a subset.",
		ArgsUsage:   "<epoch-car-path>",
		Flags: []cli.Flag{
			&cli.Int64Flag{
				Name:     "size",
				Aliases:  []string{"s"},
				Value:    31 * 1024 * 1024 * 1024, // 31 GiB
				Usage:    "Target size in bytes to chunk CARs to.",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "epoch",
				Aliases:  []string{"e"},
				Usage:    "Epoch number",
				Required: true,
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
					return fmt.Errorf("failed to open CAR: %s", err)
				}
				defer file.Close()
			}

			rd, err := carreader.New(file)
			if err != nil {
				return fmt.Errorf("failed to open CAR: %s", err)
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

			epoch := c.Int("epoch")
			maxFileSize := c.Int64("size")

			var (
				currentFileSize   int64
				currentFileNum    int
				currentFile       *os.File
				fileMutex         sync.Mutex
				currentSubsetInfo subsetInfo
				subsetLinks       []datamodel.Link
			)

			createNewFile := func() error {
				fileMutex.Lock()
				defer fileMutex.Unlock()

				if currentFile != nil {
					subsetNode, err := qp.BuildMap(ipldbindcode.Prototypes.Subset, -1, func(ma datamodel.MapAssembler) {
						qp.MapEntry(ma, "kind", qp.Int(int64(iplddecoders.KindSubset)))
						qp.MapEntry(ma, "first", qp.Int(int64(currentSubsetInfo.firstSlot)))
						qp.MapEntry(ma, "last", qp.Int(int64(currentSubsetInfo.lastSlot)))
						qp.MapEntry(ma, "blocks",
							qp.List(-1, func(la datamodel.ListAssembler) {
								for _, bl := range currentSubsetInfo.blockLinks {
									qp.ListEntry(la, qp.Link(bl))
								}
							}))
					})
					if err != nil {
						return err
					}

					cid, err := writeNode(subsetNode, currentFile)
					if err != nil {
						return err
					}

					subsetLinks = append(subsetLinks, cidlink.Link{Cid: cid})

					currentFile.Close()
				}
				currentFileNum++
				filename := fmt.Sprintf("epoch-%d-%d.car", epoch, currentFileNum)
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
				currentSubsetInfo = subsetInfo{fileName: filename, firstSlot: -1, lastSlot: -1}
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
					return fmt.Errorf("failed to write object to car file: %s, error: %w", currentFile.Name(), err)
				}
				currentFileSize += int64(len(data))
				return nil
			}

			processObject := func(owm *accum.ObjectWithMetadata) error {
				data := owm.ObjectData
				kind, err := iplddecoders.GetKind(data)
				if err != nil {
					return err
				}

				if kind == iplddecoders.KindBlock {
					block, err := iplddecoders.DecodeBlock(data)
					if err != nil {
						return err
					}

					if currentSubsetInfo.firstSlot == -1 || block.Slot < currentSubsetInfo.firstSlot {
						currentSubsetInfo.firstSlot = block.Slot
					}
					if block.Slot > currentSubsetInfo.lastSlot {
						currentSubsetInfo.lastSlot = block.Slot
					}

					currentSubsetInfo.blockLinks = append(currentSubsetInfo.blockLinks, cidlink.Link{Cid: owm.Cid})
				}

				rs, err := owm.RawSection()
				if err != nil {
					return err
				}

				return writeObject(rs)
			}

			accum := accum.NewObjectAccumulator(
				rd,
				iplddecoders.KindBlock,
				func(owm1 *accum.ObjectWithMetadata, owm2 []accum.ObjectWithMetadata) error {
					for _, owm := range owm2 {
						if err := processObject(&owm); err != nil {
							return err
						}
					}

					if err := processObject(owm1); err != nil {
						return err
					}
					return nil
				},
				iplddecoders.KindEpoch,
				iplddecoders.KindSubset,
			)

			if err := accum.Run((context.Background())); err != nil {
				return fmt.Errorf("failed to run accumulator while accumulating objects: %w", err)
			}

			epochNode, err := qp.BuildMap(ipldbindcode.Prototypes.Epoch, -1, func(ma datamodel.MapAssembler) {
				qp.MapEntry(ma, "kind", qp.Int(int64(iplddecoders.KindEpoch)))
				qp.MapEntry(ma, "epoch", qp.Int(int64(epoch)))
				qp.MapEntry(ma, "subsets",
					qp.List(-1, func(la datamodel.ListAssembler) {
						for _, sl := range subsetLinks {
							qp.ListEntry(la, qp.Link(sl))
						}
					}),
				)
			})
			if err != nil {
				return err
			}

			_, err = writeNode(epochNode, currentFile)
			if err != nil {
				return err
			}

			return nil
		},
	}
}

func writeNode(node datamodel.Node, f *os.File) (cid.Cid, error) {
	var buf bytes.Buffer
	err := dagcbor.Encode(node, &buf)
	if err != nil {
		return cid.Cid{}, err
	}

	bd := cid.V1Builder{MhLength: -1, MhType: uint64(multicodec.Sha2_256), Codec: uint64(multicodec.DagCbor)}
	cd, err := bd.Sum(buf.Bytes())
	if err != nil {
		return cid.Cid{}, err
	}

	c := []byte(cd.KeyString())
	d := buf.Bytes()

	sizeVi := appendVarint(nil, uint64(len(c))+uint64(len(d)))

	if _, err := f.Write(sizeVi); err == nil {
		if _, err := f.Write(c); err == nil {
			if _, err := f.Write(d); err != nil {
				return cid.Cid{}, err
			}

		}
	}
	return cd, nil
}

func appendVarint(tgt []byte, v uint64) []byte {
	for v > 127 {
		tgt = append(tgt, byte(v|128))
		v >>= 7
	}
	return append(tgt, byte(v))
}
