package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readahead"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func isNumeric(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func shortToKind(s string) (iplddecoders.Kind, error) {
	if isNumeric(s) {
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			panic(err)
		}
		if parsed < 0 || parsed > int64(iplddecoders.KindDataFrame) {
			return 0, fmt.Errorf("unknown kind: %d", parsed)
		}
		return iplddecoders.Kind(parsed), nil
	}
	switch s {
	case "tx", "transaction":
		return iplddecoders.KindTransaction, nil
	case "entry":
		return iplddecoders.KindEntry, nil
	case "block":
		return iplddecoders.KindBlock, nil
	case "subset":
		return iplddecoders.KindSubset, nil
	case "epoch":
		return iplddecoders.KindEpoch, nil
	case "rewards":
		return iplddecoders.KindRewards, nil
	case "dataframe":
		return iplddecoders.KindDataFrame, nil
	default:
		return 0, fmt.Errorf("unknown kind: %s", s)
	}
}

func newCmd_DumpCar() *cli.Command {
	var flagPrintFilter string
	var printID bool
	var prettyPrintTransactions bool
	var limit int
	return &cli.Command{
		Name:        "dump-car",
		Description: "Dump the contents of a CAR file",
		ArgsUsage:   "<car-path>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "filter",
				Aliases:     []string{"f", "print"},
				Usage:       "print only nodes of these kinds (comma-separated); example: --filter epoch,block",
				Destination: &flagPrintFilter,
			},

			&cli.BoolFlag{
				Name:        "id",
				Usage:       "print only the CID of the nodes",
				Destination: &printID,
			},

			&cli.BoolFlag{
				Name:        "pretty",
				Usage:       "pretty print transactions",
				Destination: &prettyPrintTransactions,
			},

			&cli.IntFlag{
				Name:        "limit",
				Usage:       "limit the number of nodes to print",
				Destination: &limit,
			},
		},
		Action: func(c *cli.Context) error {
			filter := make(intSlice, 0)
			if flagPrintFilter != "" {
				for _, v := range strings.Split(flagPrintFilter, ",") {
					v = strings.TrimSpace(v)
					v = strings.ToLower(v)
					if v == "" {
						continue
					}
					parsed, err := shortToKind(string(v))
					if err != nil {
						return fmt.Errorf("error parsing filter: %w", err)
					}
					filter = append(filter, int(parsed))
				}
			}

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

			cachingReader, err := readahead.NewCachingReaderFromReader(file, readahead.DefaultChunkSize)
			if err != nil {
				klog.Exitf("Failed to create caching reader: %s", err)
			}

			rd, err := car.NewCarReader(cachingReader)
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

			startedAt := time.Now()
			numNodesSeen := 0
			numNodesPrinted := 0
			defer func() {
				klog.Infof("Finished in %s", time.Since(startedAt))
				klog.Infof("Read %d nodes from CAR file", numNodesSeen)
			}()
			dotEvery := 100_000
			klog.Infof("A dot is printed every %d nodes", dotEvery)
			if filter.empty() {
				klog.Info("Will print all nodes of all kinds")
			} else {
				klog.Info("Will print only nodes of these kinds: ")
				for _, v := range filter {
					klog.Infof("- %s", iplddecoders.Kind(v).String())
				}
			}
			if limit > 0 {
				klog.Infof("Will print only %d nodes", limit)
			}

			for {
				if c.Context.Err() != nil {
					return c.Context.Err()
				}
				block, err := rd.Next()
				if err != nil {
					if errors.Is(err, io.EOF) {
						fmt.Println("EOF")
						break
					}
					panic(err)
				}
				numNodesSeen++
				if numNodesSeen%dotEvery == 0 {
					fmt.Print(".")
				}
				if limit > 0 && numNodesPrinted >= limit {
					break
				}
				kind := iplddecoders.Kind(block.RawData()[1])

				doPrint := filter.has(int(kind)) || filter.empty()
				if doPrint {
					fmt.Printf("\nCID=%s Multicodec=%#x Kind=%s\n", block.Cid(), block.Cid().Type(), kind)
				} else {
					continue
				}

				switch kind {
				case iplddecoders.KindTransaction:
					decoded, err := iplddecoders.DecodeTransaction(block.RawData())
					if err != nil {
						panic(err)
					}
					{
						if total, ok := decoded.Data.GetTotal(); !ok || total == 1 {
							completeData := decoded.Data.Bytes()
							{
								// verify hash (if present)
								if ha, ok := decoded.Data.GetHash(); ok {
									err := ipldbindcode.VerifyHash(completeData, ha)
									if err != nil {
										panic(err)
									}
								}
							}
							var tx solana.Transaction
							if err := bin.UnmarshalBin(&tx, completeData); err != nil {
								panic(err)
							} else if len(tx.Signatures) == 0 {
								panic("no signatures")
							}
							if doPrint {
								fmt.Println("sig=" + tx.Signatures[0].String())
								spew.Dump(decoded)
								if prettyPrintTransactions {
									fmt.Println(tx.String())
								}
								numNodesPrinted++
							}
						} else {
							if doPrint {
								fmt.Println("transaction data is split into multiple objects; skipping printing")
							}
						}
						if total, ok := decoded.Metadata.GetTotal(); !ok || total == 1 {
							completeBuffer := decoded.Metadata.Bytes()
							if ha, ok := decoded.Metadata.GetHash(); ok {
								err := ipldbindcode.VerifyHash(completeBuffer, ha)
								if err != nil {
									panic(err)
								}
							}
							if len(completeBuffer) > 0 {
								uncompressedMeta, err := decompressZstd(completeBuffer)
								if err != nil {
									panic(err)
								}
								status, err := solanatxmetaparsers.ParseAnyTransactionStatusMeta(uncompressedMeta)
								if err != nil {
									panic(err)
								}
								if doPrint {
									spew.Dump(status)
								}
							}
						} else {
							if doPrint {
								fmt.Println("transaction metadata is split into multiple objects; skipping printing")
							}
						}
					}
				case iplddecoders.KindEntry:
					decoded, err := iplddecoders.DecodeEntry(block.RawData())
					if err != nil {
						panic(err)
					}
					if doPrint {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				case iplddecoders.KindBlock:
					decoded, err := iplddecoders.DecodeBlock(block.RawData())
					if err != nil {
						panic(err)
					}
					if doPrint {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				case iplddecoders.KindSubset:
					decoded, err := iplddecoders.DecodeSubset(block.RawData())
					if err != nil {
						panic(err)
					}
					if doPrint {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				case iplddecoders.KindEpoch:
					decoded, err := iplddecoders.DecodeEpoch(block.RawData())
					if err != nil {
						panic(err)
					}
					if doPrint {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				case iplddecoders.KindRewards:
					decoded, err := iplddecoders.DecodeRewards(block.RawData())
					if err != nil {
						panic(err)
					}
					if doPrint {
						spew.Dump(decoded)
						numNodesPrinted++

						if total, ok := decoded.Data.GetTotal(); !ok || total == 1 {
							completeBuffer := decoded.Data.Bytes()
							if ha, ok := decoded.Data.GetHash(); ok {
								err := ipldbindcode.VerifyHash(completeBuffer, ha)
								if err != nil {
									panic(err)
								}
							}
							if len(completeBuffer) > 0 {
								uncompressedRewards, err := decompressZstd(completeBuffer)
								if err != nil {
									panic(err)
								}
								// try decoding as protobuf
								parsed, err := solanablockrewards.ParseRewards(uncompressedRewards)
								if err != nil {
									// TODO: add support for legacy rewards format
									fmt.Println("Rewards are not protobuf: " + err.Error())
								} else {
									spew.Dump(parsed)
								}
							}
						} else {
							fmt.Println("rewards data is split into multiple objects; skipping printing")
						}
					}
				case iplddecoders.KindDataFrame:
					decoded, err := iplddecoders.DecodeDataFrame(block.RawData())
					if err != nil {
						panic(err)
					}
					spew.Dump(decoded)
				default:
					panic("unknown kind: " + kind.String())
				}
			}
			klog.Infof("CAR file traversed successfully")
			return nil
		},
	}
}

type intSlice []int

func (s intSlice) has(v int) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

func (s intSlice) empty() bool {
	return len(s) == 0
}

var decoder, _ = zstd.NewReader(nil)

func decompressZstd(data []byte) ([]byte, error) {
	return decoder.DecodeAll(data, nil)
}
