package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gagliardetto/solana-go"
	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readahead"
	"github.com/urfave/cli/v2"
)

// blockMarkersLine is one Block node of the CAR. Markers are the raw Alpenglow
// block markers, in block order, exactly as stored.
type blockMarkersLine struct {
	Slot    int      `json:"slot"`
	BlockID *string  `json:"block_id"`
	Markers []string `json:"markers"`
}

func newCmd_XDumpBlockMarkers() *cli.Command {
	return &cli.Command{
		Name:        "x-dump-block-markers",
		Description: "Print the Alpenglow block markers of every block in a CAR as JSON lines: {slot, block_id (base58), markers (base64)}. A block without markers has an empty list.",
		ArgsUsage:   "<car-path>",
		Action: func(c *cli.Context) error {
			if c.Args().Len() != 1 {
				return fmt.Errorf("expected one CAR path")
			}
			file, err := os.Open(c.Args().First())
			if err != nil {
				return err
			}
			defer file.Close()
			reader, err := readahead.NewCachingReaderFromReader(file, readahead.DefaultChunkSize)
			if err != nil {
				return fmt.Errorf("failed to create caching reader: %w", err)
			}
			rd, err := car.NewCarReader(reader)
			if err != nil {
				return fmt.Errorf("failed to open CAR: %w", err)
			}
			out := bufio.NewWriter(os.Stdout)
			defer out.Flush()
			return dumpBlockMarkers(rd, json.NewEncoder(out))
		},
	}
}

func dumpBlockMarkers(rd *car.CarReader, enc *json.Encoder) error {
	for {
		node, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if iplddecoders.Kind(node.RawData()[1]) != iplddecoders.KindBlock {
			continue
		}
		block, err := iplddecoders.DecodeBlock(node.RawData())
		if err != nil {
			return fmt.Errorf("failed to decode block %s: %w", node.Cid(), err)
		}
		if err := enc.Encode(newBlockMarkersLine(block)); err != nil {
			return err
		}
	}
}

func newBlockMarkersLine(block *ipldbindcode.Block) blockMarkersLine {
	line := blockMarkersLine{Slot: block.Slot, Markers: []string{}}
	if id, ok := block.GetBlockID(); ok {
		s := solana.HashFromBytes(id).String()
		line.BlockID = &s
	}
	markers, _ := block.GetBlockMarkers()
	for _, marker := range markers {
		line.Markers = append(line.Markers, base64.StdEncoding.EncodeToString(marker))
	}
	return line
}
