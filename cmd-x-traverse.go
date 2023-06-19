package main

import (
	"context"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func newCmd_XTraverse() *cli.Command {
	return &cli.Command{
		Name:        "x-traverse",
		Description: "Demo of taversing the DAG of a CAR file and printing the contents of each node.",
		ArgsUsage:   "<car-path> <index-dir>",
		Before: func(c *cli.Context) error {
			return nil
		},
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			carPath := c.Args().Get(0)
			indexDir := c.Args().Get(1)

			{
				simpleIter, err := NewSimpleCarIterator(carPath, indexDir)
				if err != nil {
					panic(err)
				}
				defer simpleIter.Close()

				startedAt := time.Now()
				numSolanaBlocks := 0
				numTransactions := 0

				defer func() {
					klog.Infof("Finished in %s", time.Since(startedAt))
					klog.Infof("Read %d Solana blocks", numSolanaBlocks)
					klog.Infof("Read %d transactions", numTransactions)
				}()

				epoch, err := simpleIter.GetEpoch(context.Background())
				if err != nil {
					panic(err)
				}
				spew.Dump(epoch)

				yes := askForConfirmation("The epoch contains %d subsets. Do you want to continue?", len(epoch.Subsets))
				if !yes {
					klog.Info("Exiting...")
					return nil
				}

				for subsetIndex, subsetCID := range epoch.Subsets {
					subset, err := simpleIter.GetSubset(context.Background(), subsetCID.(cidlink.Link).Cid)
					if err != nil {
						return fmt.Errorf("failed to get subset: %w", err)
					}
					yes := askForConfirmation("	Subset %d contains %d blocks. Do you want to continue?", subsetIndex, len(subset.Blocks))
					if !yes {
						klog.Info("Exiting...")
						return nil
					}
					for blockIndex, blockCID := range subset.Blocks {
						block, err := simpleIter.GetBlock(context.Background(), blockCID.(cidlink.Link).Cid)
						if err != nil {
							return fmt.Errorf("failed to get block: %w", err)
						}
						yes := askForConfirmation("		Block %d contains %d entries. Do you want to continue?", blockIndex, len(block.Entries))
						if !yes {
							klog.Info("Exiting...")
							return nil
						}
						for entryIndex, entryCID := range block.Entries {
							entry, err := simpleIter.GetEntry(context.Background(), entryCID.(cidlink.Link).Cid)
							if err != nil {
								return fmt.Errorf("failed to get entry: %w", err)
							}
							yes := askForConfirmation("			Entry %d contains %d transactions. Do you want to continue?", entryIndex, len(entry.Transactions))
							if !yes {
								klog.Info("Exiting...")
								return nil
							}
							for _, txCID := range entry.Transactions {
								tx, err := simpleIter.GetTransaction(context.Background(), txCID.(cidlink.Link).Cid)
								if err != nil {
									return fmt.Errorf("failed to get transaction: %w", err)
								}
								spew.Dump(tx)
								{
									var transaction solana.Transaction
									{
										txBuffer, err := loadDataFromDataFrames(&tx.Data, simpleIter.GetDataFrame)
										if err != nil {
											panic(err)
										}
										if err := bin.UnmarshalBin(&transaction, txBuffer); err != nil {
											panic(err)
										} else if len(transaction.Signatures) == 0 {
											panic("no signatures")
										}
									}
									{
										fmt.Println("sig=" + transaction.Signatures[0].String())
										fmt.Println(transaction.String())
									}
									{
										metaBuffer, err := loadDataFromDataFrames(&tx.Metadata, simpleIter.GetDataFrame)
										if err != nil {
											panic(err)
										}
										if len(metaBuffer) > 0 {
											uncompressedMeta, err := decompressZstd(metaBuffer)
											if err != nil {
												panic(err)
											}
											status, err := solanatxmetaparsers.ParseTransactionStatusMeta(uncompressedMeta)
											if err != nil {
												panic(err)
											}
											spew.Dump(status)
										}
									}
								}
							}
						}
					}
				}

				if false {
					klog.Info("Iterating Solana blocks...")
					err = simpleIter.FindBlocks(context.Background(), func(_ cid.Cid, block *ipldbindcode.Block) error {
						numSolanaBlocks++
						if numSolanaBlocks%10_000 == 0 {
							fmt.Print(".")
						}
						return nil
					})
					if err != nil {
						panic(err)
					}
					took := time.Since(startedAt)
					klog.Infof("Finished iterating blocks in %s; found %d solana blocks", took, numSolanaBlocks)

					klog.Info("Iterating Solana Transactions...")
					err = simpleIter.FindTransactions(context.Background(), func(_ cid.Cid, tx *ipldbindcode.Transaction) error {
						numTransactions++
						if numTransactions%100_000 == 0 {
							fmt.Print(".")
						}
						return nil
					})
					if err != nil {
						panic(err)
					}
					took = time.Since(startedAt) - took
					klog.Infof("Finished iterating transactions in %s; found %d transactions", took, numTransactions)
				}
			}
			return nil
		},
	}
}

func askForConfirmation(message string, args ...any) bool {
	fmt.Printf(message, args...)
	fmt.Print(" [y/N]: ")
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return askForConfirmation(message)
	}
	if isStringAnyOf(response, "y", "Y", "yes", "Yes", "YES") {
		return true
	}
	return false
}

func isStringAnyOf(s string, strs ...string) bool {
	for _, str := range strs {
		if s == str {
			return true
		}
	}
	return false
}
