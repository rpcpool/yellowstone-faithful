package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipld/go-car"
	"github.com/urfave/cli/v2"
	"go.firedancer.io/radiance/cmd/radiance/car/createcar/iplddecoders"
	"go.firedancer.io/radiance/pkg/blockstore"
	"k8s.io/klog/v2"
)

func newDumpCarCmd() *cli.Command {
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
				Name:        "print",
				Usage:       "print only nodes of these kinds (comma-separated)",
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
			// parse slice of ints from flagPrintFilter
			{
				if flagPrintFilter != "" {
					for _, v := range flagPrintFilter {
						if v == ',' {
							continue
						}
						parsed, err := strconv.ParseInt(string(v), 10, 64)
						if err != nil {
							panic(err)
						}
						filter = append(filter, int(parsed))
					}
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

			rd, err := car.NewCarReader(file)
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
				klog.Info("Will print all nodes")
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
				block, err := rd.Next()
				if errors.Is(err, io.EOF) {
					fmt.Println("EOF")
					break
				}
				numNodesSeen++
				if numNodesSeen%dotEvery == 0 {
					fmt.Print(".")
				}
				if limit > 0 && numNodesPrinted >= limit {
					break
				}
				kind := iplddecoders.Kind(block.RawData()[1])
				if printID {
					fmt.Printf("\nCID=%s Multicodec=%#x Kind=%s\n", block.Cid(), block.Cid().Type(), kind)
				}

				switch kind {
				case iplddecoders.KindTransaction:
					decoded, err := iplddecoders.DecodeTransaction(block.RawData())
					if err != nil {
						panic(err)
					}
					{
						var tx solana.Transaction
						if err := bin.UnmarshalBin(&tx, decoded.Data); err != nil {
							panic(err)
						} else if len(tx.Signatures) == 0 {
							panic("no signatures")
						}
						doPrint := filter.has(int(iplddecoders.KindTransaction)) || filter.empty()
						if doPrint {
							fmt.Println("sig=" + tx.Signatures[0].String())
							spew.Dump(decoded)
							if prettyPrintTransactions {
								fmt.Println(tx.String())
							}
							numNodesPrinted++
						}
						{
							txMeta, err := blockstore.ParseTransactionStatusMeta(decoded.Metadata)
							if err != nil {
								panic(err)
							}
							if doPrint {
								spew.Dump(txMeta)
							}
						}
					}
				case iplddecoders.KindEntry:
					decoded, err := iplddecoders.DecodeEntry(block.RawData())
					if err != nil {
						panic(err)
					}
					if filter.has(int(iplddecoders.KindEntry)) || filter.empty() {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				case iplddecoders.KindBlock:
					decoded, err := iplddecoders.DecodeBlock(block.RawData())
					if err != nil {
						panic(err)
					}
					if filter.has(int(iplddecoders.KindBlock)) || filter.empty() {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				case iplddecoders.KindSubset:
					decoded, err := iplddecoders.DecodeSubset(block.RawData())
					if err != nil {
						panic(err)
					}
					if filter.has(int(iplddecoders.KindSubset)) || filter.empty() {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				case iplddecoders.KindEpoch:
					decoded, err := iplddecoders.DecodeEpoch(block.RawData())
					if err != nil {
						panic(err)
					}
					if filter.has(int(iplddecoders.KindEpoch)) || filter.empty() {
						spew.Dump(decoded)
						numNodesPrinted++
					}
				default:
					panic("unknown kind" + kind.String())
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
