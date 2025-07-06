package main

// #include <unistd.h>
import "C"

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
)

func main() {
	var carPath string
	flag.StringVar(&carPath, "car", "", "Path to the CAR file")
	flag.Parse()

	// {
	// 	total := SysTotalMemory()
	// 	free := SysFreeMemory()
	// 	used := total - free
	// 	fmt.Printf("Total memory: %s, Free memory: %s, Used memory: %s\n",
	// 		humanize.Bytes(total),
	// 		humanize.Bytes(free),
	// 		humanize.Bytes(used))
	// }
	// go monitorMemoryUsage(95.0, time.Second)

	fmt.Println("Reading CAR file:", carPath)

	file, err := os.Open(carPath)
	if err != nil {
		panic(fmt.Errorf("failed to open file %q: %w", carPath, err))
	}
	defer file.Close()
	reader, err := carreader.NewPrefetching(file)
	if err != nil {
		panic(fmt.Errorf("failed to create car reader for file %q: %w", carPath, err))
	}
	size, err := sizeOfFile(carPath)
	if err != nil {
		panic(fmt.Errorf("failed to get size of file %q: %w", carPath, err))
	}
	startedAt := time.Now()
	numRead := 0
	defer func() {
		fmt.Printf("Finished reading CAR file in %s\n", time.Since(startedAt))
		fmt.Printf("Average read speed: %s/s\n", humanize.Bytes(uint64(float64(size)/time.Since(startedAt).Seconds())))
		fmt.Printf("Total objects read: %d\n", numRead)
	}()
	lastCheckpoint := time.Now()
	for {
		numRead++
		cid, offset, buf, err := reader.NextNodeBytes()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break // No more objects to read
			}
			panic(fmt.Errorf("error reading next object: %w", err))
		}
		if numRead%1_000_000 == 0 {
			fmt.Printf("Read %d objects in %s\n", numRead, time.Since(lastCheckpoint))
			lastCheckpoint = time.Now()
		}
		if numRead%100_000_000 == 0 {
			fmt.Printf("=== Read %d objects in %s ===\n", numRead, time.Since(startedAt))
		}
		data := buf.Bytes()
		kind, err := iplddecoders.GetKind(data)
		if err != nil {
			panic(err)
		}
		_ = kind
		decoded, err := iplddecoders.DecodeAny(data)
		if err != nil {
			panic(err)
		}
		switch dec := decoded.(type) {
		case *ipldbindcode.Transaction:
			iplddecoders.PutTransaction(dec)
		case *ipldbindcode.Entry:
			iplddecoders.PutEntry(dec)
		case *ipldbindcode.Block:
			iplddecoders.PutBlock(dec)
		case *ipldbindcode.Subset:
			iplddecoders.PutSubset(dec)
		case *ipldbindcode.Epoch:
			iplddecoders.PutEpoch(dec)
		case *ipldbindcode.Rewards:
			iplddecoders.PutRewards(dec)
		case *ipldbindcode.DataFrame:
			iplddecoders.PutDataFrame(dec)
		default:
			panic(fmt.Errorf("unknown kind %s for CID %s", kind, cid))
		}
		// if kind == iplddecoders.KindBlock {
		// 	block, err := iplddecoders.DecodeBlock(node)
		// 	if err != nil {
		// 		panic(fmt.Errorf("failed to decode block: %w", err))
		// 	}
		// 	fmt.Printf("Reached solana block %d\n", block.Slot)
		// }

		_ = cid               // Use cid if needed
		_ = offset            // Use offset if needed
		reader.PutBuffer(buf) // Return the buffer to the pool
	}
}

func sizeOfFile(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info for %q: %w", path, err)
	}
	return info.Size(), nil
}

func SysTotalMemory() uint64 {
	in := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	// If this is a 32-bit system, then these fields are
	// uint32 instead of uint64.
	// So we always convert to uint64 to match signature.
	return uint64(in.Totalram) * uint64(in.Unit)
}

func SysFreeMemory() uint64 {
	in := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}
	// If this is a 32-bit system, then these fields are
	// uint32 instead of uint64.
	// So we always convert to uint64 to match signature.
	return uint64(in.Freeram) * uint64(in.Unit)
}

func ProcUsageMemory() (uint64, error) {
	in := &syscall.Rusage{}
	err := syscall.Getrusage(syscall.RUSAGE_SELF, in)
	if err != nil {
		return 0, fmt.Errorf("failed to get process memory usage: %w", err)
	}
	// Convert to bytes
	return uint64(in.Maxrss) * 1024, nil // Maxrss is in kilobytes
}

func exitIfMemUsageTooHigh(thresholdPercent float64) {
	total := SysTotalMemory()
	free := SysFreeMemory()
	used := total - free

	usedPercent := float64(used) / float64(total) * 100.0
	if usedPercent > thresholdPercent {
		procUsage, err := ProcUsageMemory()
		if err != nil {
			fmt.Printf("Failed to get process memory usage: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Memory usage is too high: %.2f%% (threshold: %.2f%%). This process is using %s of memory.\n", usedPercent, thresholdPercent, humanize.Bytes(procUsage))
		fmt.Printf("Total memory: %s, Free memory: %s, Used memory: %s\n",
			humanize.Bytes(total),
			humanize.Bytes(free),
			humanize.Bytes(used))
		fmt.Println("Exiting to prevent OOM killer.")
		os.Exit(1)
	}
}

func monitorMemoryUsage(thresholdPercent float64, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			exitIfMemUsageTooHigh(thresholdPercent)
		}
	}
}
