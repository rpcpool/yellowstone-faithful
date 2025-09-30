package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/nodetools"
	solanablockrewards "github.com/rpcpool/yellowstone-faithful/solana-block-rewards"
	diff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

func main() {
	var (
		carpath1   string
		carpath2   string
		diffFormat string
	)
	flag.StringVar(&carpath1, "car1", "", "Path to the first CAR file")
	flag.StringVar(&carpath2, "car2", "", "Path to the second CAR file")
	flag.StringVar(&diffFormat, "format", "ascii", "Output format for differences (ascii or delta)")
	flag.Parse()
	if carpath1 == "" || carpath2 == "" {
		flag.Usage()
		return
	}
	slog.Info("Comparing old-faithful CAR files",
		"car1", carpath1,
		"car2", carpath2,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	producerIDCar1 := ProducerID("car1")
	producerIDCar2 := ProducerID("car2")

	printMUTEX := &sync.Mutex{}
	mismatchCallback := func(tuples TupleSlice) {
		printMUTEX.Lock()
		defer printMUTEX.Unlock()
		fmt.Println("🚨 🚨 🚨")
		fmt.Printf("\n!!! MISMATCH DETECTED !!!\n")
		fmt.Printf("Blocks with the same SLOT have different CID hashes:\n")
		fmt.Printf("SLOT: %d\n", tuples[0].Slot)
		fmt.Printf("Hashes:\n")
		for _, t := range tuples {
			fmt.Printf("  -> From CAR '%s': Slot=%d, Hash=%s\n", t.ProducerID, t.Slot, t.Hash)
		}
		// spew.Dump(tuples)
		{
			if len(tuples) != 2 {
				panic(fmt.Sprintf("Expected exactly two tuples, but got %d", len(tuples)))
			}
			if tuples[0].ProducerID == tuples[1].ProducerID {
				panic(fmt.Sprintf("Expected tuples from different producers, but got both from '%s'", tuples[0].ProducerID))
			}
			if tuples[0].Slot != tuples[1].Slot {
				panic(fmt.Sprintf("Expected tuples with the same SLOT, but got %d and %d", tuples[0].Slot, tuples[1].Slot))
			}
			dagProducer1Wrapper, err := tuples.GetSingleByProducerID(producerIDCar1)
			if err != nil {
				panic(fmt.Sprintf("Failed to get single tuple for producer '%s': %v", producerIDCar1, err))
			}
			dagProducer2Wrapper, err := tuples.GetSingleByProducerID(producerIDCar2)
			if err != nil {
				panic(fmt.Sprintf("Failed to get single tuple for producer '%s': %v", producerIDCar2, err))
			}
			dagCar1 := dagProducer1Wrapper.Value.(*nodetools.DataAndCidSlice)
			dagCar2 := dagProducer2Wrapper.Value.(*nodetools.DataAndCidSlice)
			if dagCar1.IsEmpty() || dagCar2.IsEmpty() {
				panic(fmt.Sprintf("Expected non-empty DataAndCidSlice for both producers, but got empty for one or both: %s, %s", producerIDCar1, producerIDCar2))
			}
			// sort the CIDs for better readability.
			dagCar1.SortByCid()
			dagCar2.SortByCid()

			parsedDag1, err := dagCar1.ToParsedAndCidSlice()
			if err != nil {
				panic(fmt.Sprintf("Failed to convert DataAndCidSlice to ParsedAndCidSlice for producer '%s': %v", producerIDCar1, err))
			}
			parsedDag2, err := dagCar2.ToParsedAndCidSlice()
			if err != nil {
				panic(fmt.Sprintf("Failed to convert DataAndCidSlice to ParsedAndCidSlice for producer '%s': %v", producerIDCar2, err))
			}
			{

				block1, err := parsedDag1.BlockByCid(dagProducer1Wrapper.Hash)
				if err != nil {
					panic(fmt.Sprintf("Failed to get block by CID for producer '%s': %v", producerIDCar1, err))
				}
				if block1 == nil {
					panic(fmt.Sprintf("Expected to find block for producer '%s' with CID %s, but got nil", producerIDCar1, dagProducer1Wrapper.Hash))
				}
				block2, err := parsedDag2.BlockByCid(dagProducer2Wrapper.Hash)
				if err != nil {
					panic(fmt.Sprintf("Failed to get block by CID for producer '%s': %v", producerIDCar2, err))
				}
				if block2 == nil {
					panic(fmt.Sprintf("Expected to find block for producer '%s' with CID %s, but got nil", producerIDCar2, dagProducer2Wrapper.Hash))
				}
				spew.Dump(block1)
				spew.Dump(block2)
				{
					if block1.HasRewards() != block2.HasRewards() {
						// print a warning if the reward status differs.
						slog.Warn("Reward status differs between blocks",
							"producer1", producerIDCar1,
							"producer2", producerIDCar2,
							"hasRewards1", block1.HasRewards(),
							"hasRewards2", block2.HasRewards(),
						)
					} else {
						rewards1Cid, hasRewards1 := block1.GetRewards()
						rewards2Cid, hasRewards2 := block2.GetRewards()
						if hasRewards1 != hasRewards2 {
							slog.Warn("Reward CIDs differ between blocks",
								"producer1", producerIDCar1,
								"producer2", producerIDCar2,
								"hasRewards1", hasRewards1,
								"hasRewards2", hasRewards2,
							)
						}
						if hasRewards1 && hasRewards2 && !rewards1Cid.Equals(rewards2Cid) {
							slog.Warn("Reward CIDs differ between blocks",
								"producer1", producerIDCar1,
								"producer2", producerIDCar2,
								"rewards1Cid", rewards1Cid,
								"rewards2Cid", rewards2Cid,
							)
							{
								rewards1, err := nodetools.GetParsedRewards(parsedDag1, rewards1Cid)
								if err != nil {
									panic(fmt.Sprintf("Failed to get parsed rewards by CID %s for block %d for car1: %v", rewards1Cid, block1.Slot, err))
								}
								rewards2, err := nodetools.GetParsedRewards(parsedDag2, rewards2Cid)
								if err != nil {
									panic(fmt.Sprintf("Failed to get parsed rewards by CID %s for block %d for car2: %v", rewards2Cid, block2.Slot, err))
								}
								solanablockrewards.SortRewardsByPubkey(rewards1)
								solanablockrewards.SortRewardsByPubkey(rewards2)
								{
									rewards1Json, err := json.Marshal(rewards1)
									if err != nil {
										panic(fmt.Sprintf("Failed to marshal rewards1 to JSON: %v", err))
									}
									rewards2Json, err := json.Marshal(rewards2)
									if err != nil {
										panic(fmt.Sprintf("Failed to marshal rewards2 to JSON: %v", err))
									}
									{
										// Print the rewards data for both producers.
										fmt.Printf("Rewards for producer '%s':\n", producerIDCar1)
										fmt.Printf("  -> Rewards CID: %s\n", rewards1Cid)
										fmt.Printf("  -> Rewards Data: %s\n", string(rewards1Json))
										//
										fmt.Printf("Rewards for producer '%s':\n", producerIDCar2)
										fmt.Printf("  -> Rewards CID: %s\n", rewards2Cid)
										fmt.Printf("  -> Rewards Data: %s\n", string(rewards2Json))
									}
									differ := diff.New()
									d, err := differ.Compare(rewards1Json, (rewards2Json))
									if err != nil {
										panic(fmt.Sprintf("Failed to compare rewards JSON: %v", err))
									}
									if d.Modified() {
										fmt.Println("Rewards JSON contents differ:")
										{
											var diffString string
											switch diffFormat {
											case "ascii":
												{
													var aJson map[string]interface{}
													json.Unmarshal(rewards1Json, &aJson)

													config := formatter.AsciiFormatterConfig{
														ShowArrayIndex: true,
														Coloring:       true,
													}

													formatter := formatter.NewAsciiFormatter(aJson, config)
													diffString, err = formatter.Format(d)
													if err != nil {
														// No error can occur
													}
												}
											case "delta":
												{
													formatter := formatter.NewDeltaFormatter()
													diffString, err = formatter.Format(d)
													if err != nil {
														// No error can occur
													}
												}
											default:
												panic(fmt.Sprintf("Unknown diff format: %s", diffFormat))
											}
											fmt.Printf("Differences:\n%s\n", diffString)
										}
									} else {
										fmt.Println("Rewards JSON contents are identical.")
									}
								}

							}
						}
					}
				}
			}

		}
		fmt.Println("🚨 🚨 🚨")
	}

	matcher := NewMatcher(ctx, 2, 10_000, mismatchCallback)

	// Register producers and get their dedicated emitter functions.
	emitter1, err := matcher.RegisterProducer(producerIDCar1)
	if err != nil {
		panic(fmt.Sprintf("Fatal: Failed to register producer for first CAR file: %v", err))
	}

	emitter2, err := matcher.RegisterProducer(producerIDCar2)
	if err != nil {
		panic(fmt.Sprintf("Fatal: Failed to register producer for second CAR file: %v", err))
	}

	// Start the matcher. It no longer returns the input channel.
	errChan := matcher.Start()

	// sizeOfFile1, err := sizeOfFile(carpath1)
	// if err != nil {
	// 	slog.Error("Failed to get size of first CAR file", "error", err, "carpath", carpath1)
	// 	panic(fmt.Sprintf("Fatal: Failed to get size of first CAR file: %v", err))
	// }
	// sizeOfFile2, err := sizeOfFile(carpath2)
	// if err != nil {
	// 	slog.Error("Failed to get size of second CAR file", "error", err, "carpath", carpath2)
	// 	panic(fmt.Sprintf("Fatal: Failed to get size of second CAR file: %v", err))
	// }

	highestSlotCar1 := new(atomic.Uint64)
	highestSlotCar2 := new(atomic.Uint64)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				slog.Info("Current highest slots",
					"car1", highestSlotCar1.Load(),
					"car2", highestSlotCar2.Load(),
				)
			case <-ctx.Done():
				return
			}
		}
	}()
	numBlocksReadCar1 := new(atomic.Uint64)
	numBlocksReadCar2 := new(atomic.Uint64)
	r1, err := nodetools.NewBlockDag(carpath1, func(dag *nodetools.DataAndCidSlice) error {
		select {
		case <-ctx.Done():
			return ctx.Err() // Stop processing if the context is done
		default:
			// Continue processing the first CAR file
		}
		blocks, err := dag.Blocks()
		if err != nil {
			slog.Error("Failed to get blocks from DataAndCidSlice", "error", err, "carpath", carpath1)
			panic(fmt.Sprintf("Fatal: Failed to get blocks from DataAndCidSlice: %v", err))
		}
		if len(blocks) != 1 {
			slog.Warn("Expected exactly one block, but got more",
				"numBlocks", len(blocks),
				"carpath", carpath1,
			)
			return fmt.Errorf("expected exactly one block, but got %d", len(blocks))
		}
		blockWrapper := blocks[0]

		// Emit each block to the matcher using the emitter function.
		block := blockWrapper.Data.(*ipldbindcode.Block)
		highestSlotCar1.Store(uint64(block.Slot))
		emitter1(uint64(block.Slot), blockWrapper.Cid, dag)
		{
			// if we are ahead of the highest slot of the second CAR file, we slow down for a bit.
			if highestSlotCar1.Load() > highestSlotCar2.Load()+1000 {
				time.Sleep(50 * time.Millisecond)
			}
		}
		numBlocksReadCar1.Add(1)
		// dag.Put() // NOTE:
		return nil // No action needed for the first CAR file
	})
	if err != nil {
		slog.Error("Failed to create BlockDAG for first CAR file", "error", err, "carpath", carpath1)
		panic(fmt.Sprintf("Fatal: Failed to create BlockDAG for first CAR file: %v", err))
	}

	r2, err := nodetools.NewBlockDag(carpath2, func(dag *nodetools.DataAndCidSlice) error {
		select {
		case <-ctx.Done():
			return ctx.Err() // Stop processing if the context is done
		default:
			// Continue processing the second CAR file
		}
		blocks, err := dag.Blocks()
		if err != nil {
			slog.Error("Failed to get blocks from DataAndCidSlice", "error", err, "carpath", carpath2)
			return err
		}
		if len(blocks) != 1 {
			slog.Warn("Expected exactly one block, but got more",
				"numBlocks", len(blocks),
				"carpath", carpath2,
			)
			return fmt.Errorf("expected exactly one block, but got %d", len(blocks))
		}
		blockWrapper := blocks[0]
		// Emit each block to the matcher using the emitter function.
		block := blockWrapper.Data.(*ipldbindcode.Block)
		highestSlotCar2.Store(uint64(block.Slot))
		emitter2(uint64(block.Slot), blockWrapper.Cid, dag)
		{
			// if we are ahead of the highest slot of the first CAR file, we slow down for a bit.
			if highestSlotCar2.Load() > highestSlotCar1.Load()+1000 {
				time.Sleep(50 * time.Millisecond)
			}
		}
		numBlocksReadCar2.Add(1)
		// dag.Put() // NOTE:
		return nil // No action needed for the second CAR file
	})
	if err != nil {
		slog.Error("Failed to create BlockDAG for second CAR file", "error", err, "carpath", carpath2)
		panic(fmt.Sprintf("Fatal: Failed to create BlockDAG for second CAR file: %v", err))
	}
	if headerDiffs := cmpHeaders(r1.Header(), r2.Header()); len(headerDiffs) > 0 {
		slog.Warn("CAR headers differ ❗",
			"car1", r1.Header(),
			"car2", r2.Header(),
			"diffs", headerDiffs,
		)
	} else {
		slog.Info("CAR headers were identical ✅",
			"car1", r1.Header(),
			"car2", r2.Header(),
		)
	}

	slog.Info("Starting comparison of CAR files...",
		"car1", carpath1,
		"car2", carpath2,
	)

	readersWaitGroup := &sync.WaitGroup{}
	{
		numFinished := new(atomic.Uint64)
		// TODO: start r1, r2 iterators and feed them to the matcher.
		// DO NOT free the DataAndCidSlice automatically in the BlockDAG; leave that to the matcher when it receives and they match.
		readersWaitGroup.Add(1)
		go func() {
			defer numFinished.Add(1)
			defer readersWaitGroup.Done()

			if err := r1.Do(); err != nil {
				slog.Error("Error processing first CAR file", "error", err, "carpath", carpath1)
				return
			}
			slog.Info("Finished processing first CAR file", "carpath", carpath1)
			if numFinished.Load() == 1 {
				// If this was the last reader, we can close the error channel to signal completion.
				cancel()
			}
		}()
		readersWaitGroup.Add(1)
		go func() {
			defer numFinished.Add(1)
			defer readersWaitGroup.Done()

			if err := r2.Do(); err != nil {
				slog.Error("Error processing second CAR file", "error", err, "carpath", carpath2)
				return
			}
			slog.Info("Finished processing second CAR file", "carpath", carpath2)
			if numFinished.Load() == 1 {
				// If this was the last reader, we can close the error channel to signal completion.
				cancel()
			}
		}()
	}

	select {
	case err, ok := <-errChan:
		if ok {
			fmt.Printf("\n--- COMPARISON ENDED: BACKLOG ERROR ---\n")
			fmt.Printf("Error received from matcher: %v\n", err)
			slog.Info(
				"highest slots",
				"car1", highestSlotCar1.Load(),
				"car2", highestSlotCar2.Load(),
			)
		} else {
			fmt.Println("\n--- COMPARISON ENDED: MATCHER SHUT DOWN NORMALLY ---")
		}
	case <-ctx.Done():
		slog.Info("Comparison context cancelled or finished, shutting down matcher")
		readersWaitGroup.Wait()
		cancel()
		slog.Info("Comparison completed")
		matcher.Stop()
		{
			counts := matcher.GetCounts()
			spew.Dump(counts)
			slog.Info("Comparison completed",
				"car1", carpath1,
				"car2", carpath2,
				"numCompared", counts.Compared,
				"numMatches", counts.Matches,
				"numDiffers", counts.Differs,
				"pending", counts.Pending,
				"numBlocksReadCar1", numBlocksReadCar1.Load(),
				"numBlocksReadCar2", numBlocksReadCar2.Load(),
			)
			// sanity checks:
			if counts.Compared != numBlocksReadCar1.Load() || counts.Compared != numBlocksReadCar2.Load() {
				slog.Error("Mismatch in compared counts",
					"numCompared", counts.Compared,
					"numBlocksReadCar1", numBlocksReadCar1.Load(),
					"numBlocksReadCar2", numBlocksReadCar2.Load(),
				)
			}
			// must not have pending items at the end.
			if counts.Pending > 0 {
				slog.Error("Pending items at the end of comparison",
					"pending", counts.Pending,
				)
			}
			if counts.Compared != counts.Matches+counts.Differs {
				slog.Error("Mismatch in compared counts",
					"numCompared", counts.Compared,
					"numMatches", counts.Matches,
					"numDiffers", counts.Differs,
				)
			}
			if counts.Differs > 0 {
				slog.Warn("There were differences detected during the comparison",
					"numDiffers", counts.Differs,
				)
				// print an emoji banner.
				fmt.Println("\n🚨 Differences detected during the comparison! 🚨")
			} else {
				slog.Info("All blocks matched successfully",
					"numMatches", counts.Matches,
				)
				// print an emoji banner.
				fmt.Println("\n🎉 All blocks matched successfully! 🎉")
			}
		}
	}
}

// cmpHeaders compares two CAR headers and returns a slice of differences.
// If there are no differences, it returns an empty slice.
func cmpHeaders(h1 *car.CarHeader, h2 *car.CarHeader) []string {
	diffs := make([]string, 0)
	if h1.Version != h2.Version {
		diffs = append(diffs, fmt.Sprintf("Version mismatch: %d != %d", h1.Version, h2.Version))
	}
	if h1.Roots == nil && h2.Roots != nil || h1.Roots != nil && h2.Roots == nil {
		diffs = append(diffs, "Roots mismatch: one is nil, the other is not")
	}
	if len(h1.Roots) != len(h2.Roots) {
		diffs = append(diffs, fmt.Sprintf("Roots length mismatch: %d != %d", len(h1.Roots), len(h2.Roots)))
	} else {
		for i := range h1.Roots {
			if h1.Roots[i] != h2.Roots[i] {
				diffs = append(diffs, fmt.Sprintf("Root %d mismatch: %s != %s", i, h1.Roots[i], h2.Roots[i]))
			}
		}
	}
	return diffs
}

func sizeOfFile(path string) (int64, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info for %s: %w", path, err)
	}
	return fileInfo.Size(), nil
}
