package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/anjor/carlet"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-leb128"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car"
	carv2 "github.com/ipld/go-car/v2"
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
	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

var (
	CBOR_SHA256_DUMMY_CID = cid.MustParse("bafyreics5uul5lbtxslcigtoa5fkba7qgwu7cyb7ih7z6fzsh4lgfgraau")
	hdr                   = &car.CarHeader{
		Roots:   []cid.Cid{CBOR_SHA256_DUMMY_CID}, // placeholder
		Version: 1,
	}
	hdrSize, _     = car.HeaderSize(hdr)
	maxSectionSize = 2 << 20 // 2 MiB
)

const (
	maxLinks = 432000 / 18 // 18 subsets
	bufSize  = ((16 << 20) / 128 * 127)
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
	fileSize   uint64
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
				Value:    4 * 1024 * 1024 * 1024, // 4 GiB
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

			var (
				currentFileSize   uint64
				currentFileNum    int
				currentFile       *os.File
				bufferedWriter    *bufio.Writer
				currentSubsetInfo subsetInfo
				subsetLinks       []datamodel.Link
				carFiles          []carFile
				metadata          *splitcarfetcher.Metadata
			)

			metadata = &splitcarfetcher.Metadata{}
			headerBuf := new(bytes.Buffer)
			teeReader := io.TeeReader(file, headerBuf)

			streamBuf := bufio.NewReaderSize(teeReader, 1<<20)

			actualHeader, headerSize, err := readHeader(streamBuf)
			if err != nil {
				return fmt.Errorf("failed to read header: %w", err)
			}

			encodedHeader := base64.StdEncoding.EncodeToString(actualHeader)

			metadata.CarPieces = &carlet.CarPiecesAndMetadata{OriginalCarHeader: encodedHeader, OriginalCarHeaderSize: uint64(headerSize)}

			combinedReader := io.MultiReader(headerBuf, file)
			rd, err := carreader.New(io.NopCloser(combinedReader))
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
			maxFileSize := uint64(c.Int64("size"))
			outputDir := c.String("output-dir")
			meta := c.String("metadata")

			if outputDir == "" {
				outputDir = "."
			}

			createNewFile := func() error {
				if currentFile != nil {
					sl, err := writeSubsetNode(currentSubsetInfo, bufferedWriter)
					if err != nil {
						return fmt.Errorf("failed to write subset node: %w", err)
					}
					subsetLinks = append(subsetLinks, sl)

					cf := carFile{
						name:       fmt.Sprintf("epoch-%d-%d.car", epoch, currentFileNum),
						payloadCid: sl.(cidlink.Link).Cid,
						fileSize:   currentFileSize,
					}
					carFiles = append(carFiles, cf)

					metadata.CarPieces.CarPieces = append(
						metadata.CarPieces.CarPieces,
						carlet.CarFile{
							Name:        currentSubsetInfo.fileName,
							ContentSize: currentFileSize - hdrSize,
							HeaderSize:  hdrSize,
						})

					err = closeFile(bufferedWriter, currentFile)
					if err != nil {
						return fmt.Errorf("failed to close file: %w", err)
					}

					err = carv2.ReplaceRootsInFile(filepath.Join(outputDir, cf.name), []cid.Cid{cf.payloadCid})
					if err != nil {
						return fmt.Errorf("failed to replace root: %w", err)
					}

				}

				currentFileNum++
				filename := filepath.Join(outputDir, fmt.Sprintf("epoch-%d-%d.car", epoch, currentFileNum))
				currentFile, err = os.Create(filename)
				if err != nil {
					return fmt.Errorf("failed to create file %s: %w", filename, err)
				}

				bufferedWriter = bufio.NewWriter(currentFile)

				if err := car.WriteHeader(hdr, bufferedWriter); err != nil {
					return fmt.Errorf("failed to write header: %w", err)
				}

				// Set the currentFileSize to the size of the header
				currentFileSize = hdrSize
				currentSubsetInfo = subsetInfo{fileName: filename, firstSlot: -1, lastSlot: -1}
				return nil
			}

			writeObject := func(data []byte) error {
				_, err := bufferedWriter.Write(data)
				if err != nil {
					return fmt.Errorf("failed to write object to car file: %s, error: %w", currentFile.Name(), err)
				}
				currentFileSize += uint64(len(data))
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
				func(parent *accum.ObjectWithMetadata, children []accum.ObjectWithMetadata) error {
					if parent == nil {
						return nil
					}

					family := append(children, *parent)
					dagSize := 0

					for _, member := range family {
						dagSize += member.RawSectionSize()
					}

					if currentFile == nil || currentFileSize+uint64(dagSize) > maxFileSize || len(currentSubsetInfo.blockLinks) > maxLinks {
						klog.Infof("Creating new file, currentFileSize: %d, dagSize: %d, maxFileSize: %d, maxLinks: %d, currentSubsetInfo.blockLinks: %d", currentFileSize, dagSize, maxFileSize, maxLinks, len(currentSubsetInfo.blockLinks))
						err := createNewFile()
						if err != nil {
							return fmt.Errorf("failed to create a new file: %w", err)
						}
					}

					// owm1 is necessarily a Block
					block, err := iplddecoders.DecodeBlock(parent.ObjectData)
					if err != nil {
						return fmt.Errorf("failed to decode block: %w", err)
					}

					if currentSubsetInfo.firstSlot == -1 || block.Slot < currentSubsetInfo.firstSlot {
						currentSubsetInfo.firstSlot = block.Slot
					}
					if block.Slot > currentSubsetInfo.lastSlot {
						currentSubsetInfo.lastSlot = block.Slot
					}

					currentSubsetInfo.blockLinks = append(currentSubsetInfo.blockLinks, cidlink.Link{Cid: parent.Cid})

					err = writeBlockDag(family)
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

			sl, err := writeSubsetNode(currentSubsetInfo, bufferedWriter)
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

			_, err = writeNode(epochNode, bufferedWriter)
			if err != nil {
				return fmt.Errorf("failed to write epochNode: %w", err)
			}

			cf := carFile{
				name:       fmt.Sprintf("epoch-%d-%d.car", epoch, currentFileNum),
				payloadCid: sl.(cidlink.Link).Cid,
				fileSize:   currentFileSize,
			}

			carFiles = append(carFiles, cf)
			metadata.CarPieces.CarPieces = append(
				metadata.CarPieces.CarPieces,
				carlet.CarFile{
					Name:        currentSubsetInfo.fileName,
					ContentSize: currentFileSize - hdrSize,
					HeaderSize:  hdrSize,
				})

			err = closeFile(bufferedWriter, currentFile)
			if err != nil {
				return fmt.Errorf("failed to close file: %w", err)
			}

			err = carv2.ReplaceRootsInFile(filepath.Join(outputDir, cf.name), []cid.Cid{cf.payloadCid})
			if err != nil {
				return fmt.Errorf("failed to replace root: %w", err)
			}

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
			for idx, c := range carFiles {
				commP, paddedPieceSize, err := calcCommP(filepath.Join(outputDir, c.name))
				if err != nil {
					return fmt.Errorf("failed to calculate commP: %w", err)
				}

				err = w.Write([]string{
					c.name,
					commP.String(),
					c.payloadCid.String(),
					strconv.FormatUint(paddedPieceSize, 10),
					strconv.FormatUint(c.fileSize, 10),
				})
				if err != nil {
					return fmt.Errorf("failed to write metatadata csv: %w", err)
				}

				// it is assumed that metadata and car files are in the same order
				metadata.CarPieces.CarPieces[idx].CommP = commP
				metadata.CarPieces.CarPieces[idx].PaddedSize = paddedPieceSize
			}

			err = writeMetadata(metadata, epoch)
			if err != nil {
				return fmt.Errorf("failed to write metatadata yaml: %w", err)
			}

			return nil
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

func writeMetadata(metadata *splitcarfetcher.Metadata, epoch int) error {
	metadataFileName := fmt.Sprintf("epoch-%d-metadata.yaml", epoch)

	// Open file in append mode
	metadataFile, err := os.OpenFile(metadataFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer metadataFile.Close()

	encoder := yaml.NewEncoder(metadataFile)
	err = encoder.Encode(metadata)
	if err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	return nil
}

func readHeader(streamBuf *bufio.Reader) ([]byte, int64, error) {
	maybeHeaderLen, err := streamBuf.Peek(varintSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read header: %s", err)
	}

	hdrLen, viLen := binary.Uvarint(maybeHeaderLen)
	if hdrLen <= 0 || viLen < 0 {
		return nil, 0, fmt.Errorf("unexpected header len = %d, varint len = %d", hdrLen, viLen)
	}

	actualViLen, err := io.CopyN(io.Discard, streamBuf, int64(viLen))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to discard header varint: %s", err)
	}
	streamLen := actualViLen

	headerBuf := new(bytes.Buffer)

	actualHdrLen, err := io.CopyN(headerBuf, streamBuf, int64(hdrLen))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read header: %s", err)
	}
	streamLen += actualHdrLen

	return headerBuf.Bytes(), streamLen, nil
}

type carFileInfo struct {
	name      string
	firstSlot int64
	size      int64
}

func SortCarFiles(carFiles []string) ([]carFileInfo, error) {
	var fileInfos []carFileInfo

	for _, path := range carFiles {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open CAR file %s: %w", path, err)
		}
		defer file.Close()

		// Create a new CarReader
		cr, err := carreader.New(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create CarReader for %s: %w", path, err)
		}

		// Get the root CID
		if len(cr.Header.Roots) != 1 {
			return nil, fmt.Errorf("expected 1 root CID, got %d in file %s", len(cr.Header.Roots), path)
		}
		rootCid := cr.Header.Roots[0]

		// Read nodes until we find the one matching the root CID
		var subset *ipldbindcode.Subset
		for {
			c, _, blockData, err := cr.NextNodeBytes()
			if err != nil {
				if err == io.EOF {
					return nil, fmt.Errorf("reached end of file without finding root node in %s", path)
				}
				return nil, fmt.Errorf("failed to read node in file %s: %w", path, err)
			}

			if c == rootCid {
				// Parse the block as a Subset object
				subset, err = iplddecoders.DecodeSubset(blockData)
				if err != nil {
					return nil, fmt.Errorf("failed to decode Subset from block in file %s: %w", path, err)
				}
				break
			}
		}

		if subset == nil {
			return nil, fmt.Errorf("failed to find root node in file %s", path)
		}

		fi, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to get file info for %s: %w", path, err)
		}

		fileInfos = append(fileInfos, carFileInfo{
			name:      path,
			firstSlot: int64(subset.First),
			size:      fi.Size(),
		})
	}

	// Sort the file infos based on the firstSlot
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].firstSlot < fileInfos[j].firstSlot
	})

	return fileInfos, nil
}

func SortCarURLs(carURLs []string) ([]carFileInfo, error) {
	var urlInfos []carFileInfo

	for _, url := range carURLs {
		firstSlot, size, err := getSlotAndSizeFromURL(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get first slot from URL %s: %w", url, err)
		}

		urlInfos = append(urlInfos, carFileInfo{
			name:      url,
			firstSlot: firstSlot,
			size:      size,
		})
	}

	// Sort the URL infos based on the firstSlot
	sort.Slice(urlInfos, func(i, j int) bool {
		return urlInfos[i].firstSlot < urlInfos[j].firstSlot
	})

	return urlInfos, nil
}

func getSlotAndSizeFromURL(url string) (int64, int64, error) {
	fileSize, err := splitcarfetcher.GetContentSizeWithHeadOrZeroRange(url)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get file size: %w", err)
	}

	rootCID, err := getRootCid(url)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get root CID: %w", err)
	}

	endOffset := getEndOffset(fileSize)

	partialContent, err := fetchFromOffset(url, endOffset)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch partial content: %w", err)
	}

	cidBytes := rootCID.Bytes()
	index := bytes.LastIndex(partialContent, cidBytes)
	if index == -1 {
		return 0, 0, fmt.Errorf("CID block not found in the last 2MiB of the file")
	}
	blockData := partialContent[index-2:]
	r := bufio.NewReader(bytes.NewBuffer(blockData))
	cid, _, data, err := carreader.ReadNodeInfoWithData(r)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read node info: %w", err)
	}
	if cid != rootCID {
		return 0, 0, fmt.Errorf("expected CID %s, got %s", rootCID, cid)
	}

	// Decode the Subset
	subset, err := iplddecoders.DecodeSubset(data)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to decode Subset from block: %w", err)
	}

	return int64(subset.First), fileSize, nil
}

func getRootCid(url string) (cid.Cid, error) {
	// Make a GET request for the beginning of the file
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to create request: %w", err)
	}

	// Request only the first hdrSize bytes
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", hdrSize))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to fetch CAR file header: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return cid.Undef, fmt.Errorf("server does not support range requests")
	}

	// Read the header content
	headerContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to read header content: %w", err)
	}

	// Parse the CAR header
	rc := io.NopCloser(bytes.NewReader(headerContent))
	cr, err := carreader.New(rc)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to create CarReader: %w", err)
	}

	roots := cr.Header.Roots
	if len(roots) != 1 {
		return cid.Undef, fmt.Errorf("expected 1 root CID, got %d", len(roots))
	}
	rootCID := roots[0]

	return rootCID, nil
}

func getEndOffset(fileSize int64) int64 {
	eo := fileSize - int64(maxSectionSize)
	return max(eo, 0)
}

func fetchFromOffset(url string, offset int64) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CAR file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("server does not support range requests")
	}

	return io.ReadAll(resp.Body)
}

func calcCommP(fileName string) (cid.Cid, uint64, error) {
	r, err := os.Open(fileName)
	if err != nil {
		return cid.Undef, 0, fmt.Errorf("failed to open file: %w", err)
	}
	cp := new(commp.Calc)

	streamBuf := bufio.NewReaderSize(io.TeeReader(r, cp), bufSize)

	_, err = io.Copy(io.Discard, streamBuf)
	if err != nil {
		return cid.Undef, 0, fmt.Errorf("failed to copy stream: %w", err)
	}

	rawCommP, paddedSize, err := cp.Digest()
	if err != nil {
		return cid.Undef, 0, fmt.Errorf("failed to calculate CommP: %w", err)
	}

	commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)

	return commCid, paddedSize, err
}
