package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/rpcpool/yellowstone-faithful/bucketteer"
	"github.com/rpcpool/yellowstone-faithful/indexmeta"
	"golang.org/x/exp/mmap"
)

func main() {
	startedAt := time.Now()
	defer func() {
		fmt.Printf("took: %v\n", time.Since(startedAt))
	}()
	var numItemsToInsert int
	flag.IntVar(&numItemsToInsert, "num", 1_000_000, "num")
	flag.Parse()

	file := flag.Arg(0) // "bucketteer.bin"
	if file == "" {
		panic("no file specified")
	}

	samples := make([][64]byte, 0)
	if !fileExistsAndIsNotEmpty(file) {
		fmt.Println("File does not exist or is empty, creating it...")
		fmt.Println("Items to insert:", humanize.Comma(int64(numItemsToInsert)))
		totalWriteStartedAt := time.Now()
		buWr, err := bucketteer.NewWriter(file)
		if err != nil {
			panic(err)
		}
		defer buWr.Close()
		tookBatch := time.Duration(0)
		for i := 1; i <= numItemsToInsert; i++ {
			sig := newRandomSignature()
			startedSet := time.Now()
			buWr.Put(sig)
			tookBatch += time.Since(startedSet)
			if i%100_000 == 0 {
				fmt.Print(".")
				samples = append(samples, sig)
			}
			if i%1_000_000 == 0 {
				fmt.Print(humanize.Comma(int64(i)))
				fmt.Printf(
					" · took: %v (%s per item)\n",
					tookBatch,
					tookBatch/time.Duration(1_000_000),
				)
				tookBatch = 0
			}
		}

		fmt.Println("writing to file...")
		writeStartedAt := time.Now()
		_, err = buWr.Seal(indexmeta.Meta{})
		if err != nil {
			panic(err)
		}
		fmt.Println("writing to file took:", time.Since(writeStartedAt))
		fmt.Println("total write took:", time.Since(totalWriteStartedAt))
	}
	mmr, err := mmap.Open(file)
	if err != nil {
		panic(err)
	}
	defer mmr.Close()
	buRd, err := bucketteer.NewReader(mmr)
	if err != nil {
		panic(err)
	}
	spew.Dump(buRd.Meta())
	if len(samples) > 0 {
		fmt.Println("testing search with samples from the inserted signatures...")
		tookBatch := time.Duration(0)
		for _, sig := range samples {
			startedSearch := time.Now()
			found, err := buRd.Has(sig)
			if err != nil {
				panic(err)
			}
			if !found {
				panic("not found")
			}
			tookBatch += time.Since(startedSearch)
		}
		fmt.Println("\n"+"    num samples:", len(samples))
		fmt.Println("    search took:", tookBatch)
		fmt.Println("avg search took:", tookBatch/time.Duration(len(samples)))
	}
	if true {
		// now search for random signatures that are not in the Bucketteer:
		numSearches := 100_000_000
		fmt.Println(
			"testing search for random signatures that are not in the Bucketteer (numSearches:",
			humanize.Comma(int64(numSearches)),
			")...",
		)
		tookBatch := time.Duration(0)
		for i := 1; i <= numSearches; i++ {
			sig := newRandomSignature()
			startedSearch := time.Now()
			found, err := buRd.Has(sig)
			if err != nil {
				panic(err)
			}
			if found {
				panic("found")
			}
			tookBatch += time.Since(startedSearch)
			if i%100_000 == 0 {
				fmt.Print(".")
			}
		}
		fmt.Println("\n"+" num candidates:", humanize.Comma(int64(numSearches)))
		fmt.Println("    search took:", tookBatch)
		fmt.Println("avg search took:", tookBatch/time.Duration(numSearches))
	}
}

func newRandomSignature() [64]byte {
	var sig [64]byte
	rand.Read(sig[:])
	return sig
}

func fileExistsAndIsNotEmpty(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		panic(err)
	}
	if info.Size() == 0 {
		return false
	}
	return true
}
