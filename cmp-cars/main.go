package main

import (
	"context"
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
)

func main() {
	var (
		carpath1 string
		carpath2 string
	)
	flag.StringVar(&carpath1, "car1", "", "Path to the first CAR file")
	flag.StringVar(&carpath2, "car2", "", "Path to the second CAR file")
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

	printMUTEX := &sync.Mutex{}
	mismatchCallback := func(tuples []Tuple) {
		printMUTEX.Lock()
		defer printMUTEX.Unlock()
		fmt.Println("🚨 🚨 🚨")
		fmt.Printf("\n!!! MISMATCH DETECTED !!!\n")
		fmt.Printf("Blocks with the same SLOT have different CID hashes:\n")
		fmt.Printf("SLOT: %d\n", tuples[0].Slot)
		fmt.Printf("Hashes:\n")
		for _, t := range tuples {
			fmt.Printf("  -> From CAR '%s': Slot=%d, Hash=%s, Value='%s'\n", t.ProducerID, t.Slot, t.Hash, spew.Sdump(t.Value))
		}
		spew.Dump(tuples)
		fmt.Println("🚨 🚨 🚨")
	}

	matcher := NewMatcher(ctx, 2, 10_000, mismatchCallback)

	// Register producers and get their dedicated emitter functions.
	emitter1, err := matcher.RegisterProducer("car1")
	if err != nil {
		panic(fmt.Sprintf("Fatal: Failed to register producer for first CAR file: %v", err))
	}

	emitter2, err := matcher.RegisterProducer("car2")
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
		for _, wrapper := range blocks {
			// Emit each block to the matcher using the emitter function.
			block := wrapper.Data.(*ipldbindcode.Block)
			highestSlotCar1.Store(uint64(block.Slot))
			emitter1(uint64(block.Slot), wrapper.Cid, block)
			{
				// if we are ahead of the highest slot of the second CAR file, we slow down for a bit.
				if highestSlotCar1.Load() > highestSlotCar2.Load()+1000 {
					time.Sleep(50 * time.Millisecond)
				}
			}
			numBlocksReadCar1.Add(1)
		}
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
		for _, wrapper := range blocks {
			// Emit each block to the matcher using the emitter function.
			block := wrapper.Data.(*ipldbindcode.Block)
			highestSlotCar2.Store(uint64(block.Slot))
			emitter2(uint64(block.Slot), wrapper.Cid, block)
			{
				// if we are ahead of the highest slot of the first CAR file, we slow down for a bit.
				if highestSlotCar2.Load() > highestSlotCar1.Load()+1000 {
					time.Sleep(50 * time.Millisecond)
				}
			}
			numBlocksReadCar2.Add(1)
		}
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
