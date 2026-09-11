// Command dup-sig-check streams one or more CAR files and reports any
// transaction signature that appears in more than one transaction.
//
// This is a diagnostic for the `sig_to_cid` "hash collision" failure seen when
// running `faithful-cli index all` on CARs that contain duplicate signatures
// (e.g. testnet epochs affected by cluster restarts). The per-bucket perfect
// hash in compactindexsized can never separate two byte-identical keys, so a
// single duplicated 64-byte signature makes bucket sealing fail. This tool
// tells you exactly which signatures are duplicated and in which slots, so you
// can decide whether the duplication is expected before re-indexing with
// `--dedup-txs`.
//
// Usage:
//
//	dup-sig-check [--out report.tsv] [--max-print 50] <epoch.car> [more.car ...]
//
// It makes two streaming passes over the CAR(s):
//   - Pass 1 hashes every signature (xxHash64) into a counting set to find
//     which hashes occur more than once. Memory is bounded by the number of
//     distinct transactions (~9 bytes/tx of map payload, plus Go map overhead;
//     budget a few GB for a ~200M-tx epoch).
//   - Pass 2 collects the full signatures and slots only for those suspect
//     hashes, then confirms byte-for-byte equality (dropping the rare 64-bit
//     hash false positives) and reports the real duplicates.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	"github.com/rpcpool/yellowstone-faithful/tooling"
)

func main() {
	var (
		outPath  string
		maxPrint int
	)
	flag.StringVar(&outPath, "out", "", "optional path to write the full duplicate report as TSV (signature<TAB>count<TAB>slot,slot,...)")
	flag.IntVar(&maxPrint, "max-print", 50, "maximum number of duplicate signatures to print to stdout (0 = print all)")
	flag.Parse()

	carPaths := flag.Args()
	if len(carPaths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dup-sig-check [--out report.tsv] [--max-print 50] <epoch.car> [more.car ...]")
		os.Exit(2)
	}

	// Pass 1: find signature-hashes that occur more than once.
	fmt.Fprintln(os.Stderr, "Pass 1/2: scanning for duplicate signature hashes...")
	seen := make(map[uint64]uint8) // xxhash(sig) -> saturating count (1 or 2)
	var totalTxs uint64
	err := streamTxs(carPaths, func(sig solana.Signature, _ uint64) {
		totalTxs++
		h := xxhash.Sum64(sig[:])
		if c := seen[h]; c < 2 {
			seen[h] = c + 1
		}
	})
	if err != nil {
		fatal(err)
	}

	suspects := make(map[uint64]struct{})
	for h, c := range seen {
		if c >= 2 {
			suspects[h] = struct{}{}
		}
	}
	seen = nil // free the large pass-1 map before pass 2
	fmt.Fprintf(os.Stderr, "Pass 1 done: %s transactions, %s suspect signature-hash(es).\n",
		humanize.Comma(int64(totalTxs)), humanize.Comma(int64(len(suspects))))

	if len(suspects) == 0 {
		fmt.Println("No duplicate signatures found. This CAR should index cleanly.")
		return
	}

	// Pass 2: collect full signatures + slots for the suspect hashes only.
	fmt.Fprintln(os.Stderr, "Pass 2/2: collecting slots for duplicated signatures...")
	occurrences := make(map[solana.Signature][]uint64)
	err = streamTxs(carPaths, func(sig solana.Signature, slot uint64) {
		if _, ok := suspects[xxhash.Sum64(sig[:])]; ok {
			occurrences[sig] = append(occurrences[sig], slot)
		}
	})
	if err != nil {
		fatal(err)
	}

	// Keep only real duplicates (len >= 2); a suspect hash with a single full
	// signature was a 64-bit hash false positive.
	dups := make([]dupSig, 0, len(occurrences))
	var extraOccurrences uint64
	for sig, slots := range occurrences {
		if len(slots) < 2 {
			continue
		}
		sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
		dups = append(dups, dupSig{sig: sig, slots: slots})
		extraOccurrences += uint64(len(slots) - 1)
	}
	// Most-duplicated first, then by signature for a stable order.
	sort.Slice(dups, func(i, j int) bool {
		if len(dups[i].slots) != len(dups[j].slots) {
			return len(dups[i].slots) > len(dups[j].slots)
		}
		return dups[i].sig.String() < dups[j].sig.String()
	})

	fmt.Println()
	fmt.Printf("Total transactions scanned:      %s\n", humanize.Comma(int64(totalTxs)))
	fmt.Printf("Duplicated signatures:           %s\n", humanize.Comma(int64(len(dups))))
	fmt.Printf("Extra (redundant) occurrences:   %s\n", humanize.Comma(int64(extraOccurrences)))
	fmt.Println()

	if len(dups) == 0 {
		fmt.Println("All suspect hashes were false positives; no byte-identical duplicate signatures.")
		return
	}

	limit := len(dups)
	if maxPrint > 0 && maxPrint < limit {
		limit = maxPrint
	}
	for i := 0; i < limit; i++ {
		d := dups[i]
		fmt.Printf("%s  x%d  slots=%v\n", d.sig.String(), len(d.slots), d.slots)
	}
	if limit < len(dups) {
		fmt.Printf("... and %s more (use --out to write the full list, --max-print 0 to print all)\n",
			humanize.Comma(int64(len(dups)-limit)))
	}

	if outPath != "" {
		if err := writeReport(outPath, dups); err != nil {
			fatal(fmt.Errorf("failed to write report to %s: %w", outPath, err))
		}
		fmt.Fprintf(os.Stderr, "Wrote full report (%s duplicate signatures) to %s\n",
			humanize.Comma(int64(len(dups))), outPath)
	}
}

// streamTxs makes one streaming pass over the given CAR files, invoking fn with
// the first signature and slot of every transaction node.
func streamTxs(carPaths []string, fn func(sig solana.Signature, slot uint64)) error {
	rd, err := readasonecar.NewFromFilepaths(carPaths...)
	if err != nil {
		return fmt.Errorf("failed to open CAR(s): %w", err)
	}
	defer rd.Close()

	totalSize := rd.TotalSize()
	startedAt := time.Now()
	var nodes uint64

	for {
		offset, ok := rd.GetGlobalOffsetForNextRead()
		if !ok {
			break
		}
		_, _, buf, err := rd.NextNodeBytes()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("failed to read next node: %w", err)
		}
		raw := buf.Bytes()
		if len(raw) >= 2 && iplddecoders.Kind(raw[1]) == iplddecoders.KindTransaction {
			txNode, err := iplddecoders.DecodeTransaction(raw)
			if err != nil {
				carreader.PutBuffer(buf)
				return fmt.Errorf("failed to decode transaction: %w", err)
			}
			sig, err := tooling.ReadFirstSignature(txNode.Data.Bytes())
			if err != nil {
				carreader.PutBuffer(buf)
				return fmt.Errorf("failed to read signature: %w", err)
			}
			fn(sig, uint64(txNode.Slot))
		}
		carreader.PutBuffer(buf)

		nodes++
		if nodes%1_000_000 == 0 && totalSize > 0 {
			pct := float64(offset) / float64(totalSize) * 100
			fmt.Fprintf(os.Stderr, "\r  %s nodes [%.1f%%] in %s   ",
				humanize.Comma(int64(nodes)), pct, time.Since(startedAt).Truncate(time.Second))
		}
	}
	fmt.Fprintf(os.Stderr, "\r  %s nodes [100.0%%] in %s          \n",
		humanize.Comma(int64(nodes)), time.Since(startedAt).Truncate(time.Second))
	return nil
}

// dupSig is a signature that appears in more than one transaction, with the
// sorted list of slots it was found in.
type dupSig struct {
	sig   solana.Signature
	slots []uint64
}

func writeReport(path string, dups []dupSig) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, "signature\tcount\tslots"); err != nil {
		return err
	}
	for _, d := range dups {
		if _, err := fmt.Fprintf(f, "%s\t%d\t", d.sig.String(), len(d.slots)); err != nil {
			return err
		}
		for i, s := range d.slots {
			if i > 0 {
				if _, err := fmt.Fprint(f, ","); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(f, "%d", s); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(f); err != nil {
			return err
		}
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
