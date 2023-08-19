package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"golang.org/x/exp/mmap"
	"k8s.io/klog/v2"
)

// openIndexStorage open a compactindex from a local file, or from a remote URL.
// Supported protocols are:
// - http://
// - https://
func openIndexStorage(ctx context.Context, where string) (ReaderAtCloser, error) {
	where = strings.TrimSpace(where)
	if strings.HasPrefix(where, "http://") || strings.HasPrefix(where, "https://") {
		klog.Infof("opening file from %q as HTTP remote file", where)
		rac, err := remoteHTTPFileAsIoReaderAt(ctx, where)
		if err != nil {
			return nil, fmt.Errorf("failed to open index file: %w", err)
		}
		return &readCloserWrapper{
			rac:  rac,
			name: where,
		}, nil
	}
	// TODO: add support for IPFS gateways.
	// TODO: add support for Filecoin gateways.
	rac, err := mmap.Open(where)
	if err != nil {
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	return &readCloserWrapper{
		rac:  rac,
		name: where,
	}, nil
}

func openCarStorage(ctx context.Context, where string) (*carv2.Reader, ReaderAtCloser, error) {
	where = strings.TrimSpace(where)
	if strings.HasPrefix(where, "http://") || strings.HasPrefix(where, "https://") {
		rem, err := remoteHTTPFileAsIoReaderAt(ctx, where)
		return nil, rem, err
	}
	// TODO: add support for IPFS gateways.
	// TODO: add support for Filecoin gateways.

	carReader, err := carv2.OpenReader(where)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open CAR file: %w", err)
	}
	return carReader, nil, nil
}

func readSectionFromReaderAt(reader ReaderAtCloser, offset uint64, length uint64) ([]byte, error) {
	data := make([]byte, length)
	_, err := reader.ReadAt(data, int64(offset))
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readNodeFromReaderAt(reader ReaderAtCloser, wantedCid cid.Cid, offset uint64) ([]byte, error) {
	// read MaxVarintLen64 bytes
	lenBuf := make([]byte, binary.MaxVarintLen64)
	_, err := reader.ReadAt(lenBuf, int64(offset))
	if err != nil {
		return nil, err
	}
	// read uvarint
	dataLen, n := binary.Uvarint(lenBuf)
	offset += uint64(n)
	if dataLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
		return nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}
	data := make([]byte, dataLen)
	_, err = reader.ReadAt(data, int64(offset))
	if err != nil {
		return nil, err
	}

	n, gotCid, err := cid.CidFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// verify that the CID we read matches the one we expected.
	if !gotCid.Equals(wantedCid) {
		return nil, fmt.Errorf("CID mismatch: expected %s, got %s", wantedCid, gotCid)
	}
	return data[n:], nil
}

type GetBlockResponse struct {
	BlockHeight       *uint64                  `json:"blockHeight"`
	BlockTime         *uint64                  `json:"blockTime"`
	Blockhash         string                   `json:"blockhash"`
	ParentSlot        uint64                   `json:"parentSlot"`
	PreviousBlockhash string                   `json:"previousBlockhash"`
	Rewards           any                      `json:"rewards"` // TODO: use same format as solana
	Transactions      []GetTransactionResponse `json:"transactions"`
}

type GetTransactionResponse struct {
	// TODO: use same format as solana
	Blocktime   *uint64            `json:"blockTime,omitempty"`
	Meta        any                `json:"meta"`
	Slot        *uint64            `json:"slot,omitempty"`
	Transaction []any              `json:"transaction"`
	Version     any                `json:"version"`
	Position    uint64             `json:"-"` // TODO: enable this
	Signatures  []solana.Signature `json:"-"` // TODO: enable this
}

type GetVersionResponse struct {
  FeatureSet uint64 `json:"feature-set"`
  SolanaCore string `json:"solana-core"`
}

func loadDataFromDataFrames(
	firstDataFrame *ipldbindcode.DataFrame,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) ([]byte, error) {
	dataBuffer := new(bytes.Buffer)
	allFrames, err := getAllFramesFromDataFrame(firstDataFrame, dataFrameGetter)
	if err != nil {
		return nil, err
	}
	for _, frame := range allFrames {
		dataBuffer.Write(frame.Bytes())
	}
	// verify the data hash (if present)
	bufHash, ok := firstDataFrame.GetHash()
	if !ok {
		return dataBuffer.Bytes(), nil
	}
	err = ipldbindcode.VerifyHash(dataBuffer.Bytes(), bufHash)
	if err != nil {
		return nil, err
	}
	return dataBuffer.Bytes(), nil
}

func getAllFramesFromDataFrame(
	firstDataFrame *ipldbindcode.DataFrame,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) ([]*ipldbindcode.DataFrame, error) {
	frames := []*ipldbindcode.DataFrame{firstDataFrame}
	// get the next data frames
	next, ok := firstDataFrame.GetNext()
	if !ok || len(next) == 0 {
		return frames, nil
	}
	for _, cid := range next {
		nextDataFrame, err := dataFrameGetter(context.Background(), cid.(cidlink.Link).Cid)
		if err != nil {
			return nil, err
		}
		nextFrames, err := getAllFramesFromDataFrame(nextDataFrame, dataFrameGetter)
		if err != nil {
			return nil, err
		}
		frames = append(frames, nextFrames...)
	}
	return frames, nil
}

func parseTransactionAndMetaFromNode(
	transactionNode *ipldbindcode.Transaction,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) (tx solana.Transaction, meta any, _ error) {
	{
		transactionBuffer, err := loadDataFromDataFrames(&transactionNode.Data, dataFrameGetter)
		if err != nil {
			return solana.Transaction{}, nil, err
		}
		if err := bin.UnmarshalBin(&tx, transactionBuffer); err != nil {
			klog.Errorf("failed to unmarshal transaction: %v", err)
			return solana.Transaction{}, nil, err
		} else if len(tx.Signatures) == 0 {
			klog.Errorf("transaction has no signatures")
			return solana.Transaction{}, nil, err
		}
	}

	{
		metaBuffer, err := loadDataFromDataFrames(&transactionNode.Metadata, dataFrameGetter)
		if err != nil {
			return solana.Transaction{}, nil, err
		}
		if len(metaBuffer) > 0 {
			uncompressedMeta, err := decompressZstd(metaBuffer)
			if err != nil {
				klog.Errorf("failed to decompress metadata: %v", err)
				return
			}
			status, err := solanatxmetaparsers.ParseAnyTransactionStatusMeta(uncompressedMeta)
			if err != nil {
				klog.Errorf("failed to parse metadata: %v", err)
				return
			}
			meta = status
		}
	}
	return
}
