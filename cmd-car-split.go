package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-leb128"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/schema"
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

type carFile struct {
	name       string
	commP      cid.Cid
	payloadCid cid.Cid
	paddedSize uint64
	fileSize   int64
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
			&cli.StringFlag{
				Name:     "metadata",
				Aliases:  []string{"m"},
				Value:    "metadata.csv",
				Required: false,
				Usage:    "Filename for metadata. Defaults to metadata.csv",
			},
			&cli.StringFlag{
				Name:     "output-dir",
				Aliases:  []string{"o"},
				Usage:    "Output directory",
				Required: false,
				Value:    ".",
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
					return fmt.Errorf("failed to open CAR: %w", err)
				}
				defer file.Close()
			}

			rd, err := carreader.New(file)
			if err != nil {
				return fmt.Errorf("failed to open CAR: %w", err)
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
			outputDir := c.String("output-dir")
			meta := c.String("metadata")

			if outputDir == "" {
				outputDir = "."
			}

			cp := new(commp.Calc)

			var (
				currentFileSize   int64
				currentFileNum    int
				currentFile       *os.File
				bufferedWriter    *bufio.Writer
				currentSubsetInfo subsetInfo
				subsetLinks       []datamodel.Link
				writer            io.Writer
				carFiles          []carFile
			)

			createNewFile := func() error {

				if currentFile != nil {
					sl, err := writeSubsetNode(currentSubsetInfo, writer)
					if err != nil {
						return fmt.Errorf("failed to write subset node: %w", err)
					}
					subsetLinks = append(subsetLinks, sl)

					rawCommP, ps, err := cp.Digest()
					if err != nil {
						return fmt.Errorf("failed to calculate commp digest: %w", err)
					}

					commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
					if err != nil {
						return fmt.Errorf("failed to calculate commitment to cid: %w", err)
					}

					carFiles = append(carFiles, carFile{name: fmt.Sprintf("epoch-%d-%d.car", epoch, currentFileNum), commP: commCid, payloadCid: sl.(cidlink.Link).Cid, paddedSize: ps, fileSize: currentFileSize})

					err = closeFile(bufferedWriter, currentFile)
					if err != nil {
						return fmt.Errorf("failed to close file: %w", err)
					}

					cp.Reset()
				}

				currentFileNum++
				filename := filepath.Join(outputDir, fmt.Sprintf("epoch-%d-%d.car", epoch, currentFileNum))
				currentFile, err = os.Create(filename)
				if err != nil {
					return fmt.Errorf("failed to create file %s: %w", filename, err)
				}

				bufferedWriter = bufio.NewWriter(currentFile)
				writer = io.MultiWriter(bufferedWriter, cp)

				// Write the header
				_, err = io.WriteString(writer, nulRootCarHeader)
				if err != nil {
					return fmt.Errorf("failed to write header: %w", err)
				}

				// Set the currentFileSize to the size of the header
				currentFileSize = int64(len(nulRootCarHeader))
				currentSubsetInfo = subsetInfo{fileName: filename, firstSlot: -1, lastSlot: -1}
				return nil
			}

			writeObject := func(data []byte) error {
				_, err := writer.Write(data)
				if err != nil {
					return fmt.Errorf("failed to write object to car file: %s, error: %w", currentFile.Name(), err)
				}
				currentFileSize += int64(len(data))
				return nil
			}

			writeBlockDag := func(blockDag []accum.ObjectWithMetadata) error {
				for _, owm := range blockDag {
					rs, err := owm.RawSection()
					if err != nil {
						return fmt.Errorf("failed to get raw section: %w", err)
					}

					err = writeObject(rs)
					if err != nil {
						return fmt.Errorf("failed to write object: %w", err)
					}
				}

				return nil
			}

			accum := accum.NewObjectAccumulator(
				rd,
				iplddecoders.KindBlock,
				func(owm1 *accum.ObjectWithMetadata, owm2 []accum.ObjectWithMetadata) error {
					if owm1 == nil {
						return nil
					}

					owms := append(owm2, *owm1)
					dagSize := 0

					for _, owm := range owms {
						dagSize += owm.RawSectionSize()
					}

					if currentFile == nil || currentFileSize+int64(dagSize) > maxFileSize {
						err := createNewFile()
						if err != nil {
							return fmt.Errorf("failed to create a new file: %w", err)
						}
					}

					// owm1 is necessarily a Block
					block, err := iplddecoders.DecodeBlock(owm1.ObjectData)
					if err != nil {
						return fmt.Errorf("failed to decode block: %w", err)
					}

					if currentSubsetInfo.firstSlot == -1 || block.Slot < currentSubsetInfo.firstSlot {
						currentSubsetInfo.firstSlot = block.Slot
					}
					if block.Slot > currentSubsetInfo.lastSlot {
						currentSubsetInfo.lastSlot = block.Slot
					}

					currentSubsetInfo.blockLinks = append(currentSubsetInfo.blockLinks, cidlink.Link{Cid: owm1.Cid})

					err = writeBlockDag(owms)
					if err != nil {
						return fmt.Errorf("failed to write block dag to file: %w", err)
					}

					return nil
				},
				iplddecoders.KindEpoch,
				iplddecoders.KindSubset,
			)

			if err := accum.Run((context.Background())); err != nil {
				return fmt.Errorf("failed to run accumulator while accumulating objects: %w", err)
			}

			sl, err := writeSubsetNode(currentSubsetInfo, writer)
			if err != nil {
				return fmt.Errorf("failed to write subset node: %w", err)
			}
			subsetLinks = append(subsetLinks, sl)

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
				return fmt.Errorf("failed to construct epochNode: %w", err)
			}

			_, err = writeNode(epochNode, writer)
			if err != nil {
				return fmt.Errorf("failed to write epochNode: %w", err)
			}

			rawCommP, ps, err := cp.Digest()
			if err != nil {
				return fmt.Errorf("failed to calculate commp digest: %w", err)
			}

			commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
			if err != nil {
				return fmt.Errorf("failed to calculate commitment to cid: %w", err)
			}

			carFiles = append(carFiles, carFile{name: fmt.Sprintf("epoch-%d-%d.car", epoch, currentFileNum), commP: commCid, payloadCid: sl.(cidlink.Link).Cid, paddedSize: ps, fileSize: currentFileSize})

			f, err := os.Create(meta)
			defer f.Close()
			if err != nil {
				return err
			}

			w := csv.NewWriter(f)
			err = w.Write([]string{"car file", "piece cid", "payload cid", "padded piece size", "file size"})
			if err != nil {
				return err
			}
			defer w.Flush()
			for _, c := range carFiles {
				err = w.Write([]string{
					c.name,
					c.commP.String(),
					c.payloadCid.String(),
					strconv.FormatUint(c.paddedSize, 10),
					strconv.FormatInt(c.fileSize, 10),
				})
			}

			return closeFile(bufferedWriter, currentFile)
		},
	}
}

func writeSubsetNode(currentSubsetInfo subsetInfo, writer io.Writer) (datamodel.Link, error) {
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
		return nil, fmt.Errorf("failed to write a subsetNode: %w", err)
	}

	cid, err := writeNode(subsetNode, writer)
	if err != nil {
		return nil, fmt.Errorf("failed to write a subsetNode: %w", err)
	}

	return cidlink.Link{Cid: cid}, nil
}

func closeFile(bufferedWriter *bufio.Writer, currentFile *os.File) error {
	err := bufferedWriter.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	err = currentFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}
	return nil
}

func writeNode(node datamodel.Node, w io.Writer) (cid.Cid, error) {
	node = node.(schema.TypedNode).Representation()
	var buf bytes.Buffer
	err := dagcbor.Encode(node, &buf)
	if err != nil {
		return cid.Cid{}, err
	}

	data := buf.Bytes()

	bd := cid.V1Builder{MhLength: -1, MhType: uint64(multicodec.Sha2_256), Codec: uint64(multicodec.DagCbor)}
	cd, err := bd.Sum(data)
	if err != nil {
		return cid.Cid{}, err
	}

	c := cd.Bytes()

	sizeVi := leb128.FromUInt64(uint64(len(c)) + uint64(len(data)))

	if _, err := w.Write(sizeVi); err == nil {
		if _, err := w.Write(c); err == nil {
			if _, err := w.Write(data); err != nil {
				return cid.Cid{}, err
			}

		}
	}
	return cd, nil
}
