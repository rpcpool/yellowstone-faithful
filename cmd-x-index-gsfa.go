package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/gsfa"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_Index_gsfa() *cli.Command {
	return &cli.Command{
		Name:        "gsfa",
		Description: "Create GSFA index from a CAR file",
		ArgsUsage:   "<car-path> <gsfa-index-dir>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name:  "flush-every",
				Usage: "flush every N transactions",
				Value: 500_000,
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

			gsfaIndexDir := c.Args().Get(1)

			flushEvery := c.Uint64("flush-every")

			accu, err := gsfa.NewGsfaWriter(
				gsfaIndexDir,
				flushEvery,
			)
			if err != nil {
				return fmt.Errorf("error while opening gsfa index writer: %w", err)
			}
			defer func() {
				if err := accu.Flush(); err != nil {
					klog.Errorf("Error while flushing: %s", err)
				}
				if err := accu.Close(); err != nil {
					klog.Errorf("Error while closing: %s", err)
				}
			}()

			startedAt := time.Now()
			numTransactionsSeen := 0
			defer func() {
				klog.Infof("Finished in %s", time.Since(startedAt))
				klog.Infof("Indexed %d transactions", numTransactionsSeen)
			}()
			dotEvery := 100_000
			klog.Infof("A dot is printed every %d transactions", dotEvery)

			for {
				block, err := rd.Next()
				if errors.Is(err, io.EOF) {
					fmt.Println("EOF")
					break
				}
				kind := iplddecoders.Kind(block.RawData()[1])

				switch kind {
				case iplddecoders.KindTransaction:
					numTransactionsSeen++
					if numTransactionsSeen%dotEvery == 0 {
						fmt.Print(".")
					}
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
										klog.Exitf("Error while verifying hash for %s: %s", block.Cid(), err)
									}
								}
							}
							var tx solana.Transaction
							if err := bin.UnmarshalBin(&tx, completeData); err != nil {
								klog.Exitf("Error while unmarshaling transaction from nodex %s: %s", block.Cid(), err)
							} else if len(tx.Signatures) == 0 {
								klog.Exitf("Error while unmarshaling transaction from nodex %s: no signatures", block.Cid())
							}
							sig := tx.Signatures[0]

							err = accu.Push(sig, tx.Message.AccountKeys)
							if err != nil {
								klog.Exitf("Error while pushing to gsfa index: %s", err)
							}
						} else {
							klog.Warningf("Transaction data is split into multiple objects for %s; skipping", block.Cid())
						}
					}
				default:
					continue
				}
			}
			klog.Infof("Success: GSFA index created at %s", gsfaIndexDir)
			return nil
		},
	}
}
