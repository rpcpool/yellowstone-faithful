package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ipfs/go-cid"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"golang.org/x/exp/mmap"
	"k8s.io/klog/v2"
)

func isHTTP(where string) bool {
	return strings.HasPrefix(where, "http://") || strings.HasPrefix(where, "https://")
}

// openIndexStorage open a compactindex from a local file, or from a remote URL.
// Supported protocols are:
// - http://
// - https://
func openIndexStorage(
	ctx context.Context,
	where string,
	useMmapForLocalIndexes bool,
) (carreader.ReaderAtCloser, error) {
	where = strings.TrimSpace(where)
	if isHTTP(where) {
		klog.Infof("opening index file from %q as HTTP remote file", where)
		rac, size, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(ctx, where)
		if err != nil {
			return nil, fmt.Errorf("failed to open remote index file %q: %w", where, err)
		}
		if !klog.V(5).Enabled() {
			return rac, nil
		}
		return &readCloserWrapper{
			rac:      rac,
			name:     where,
			isRemote: true,
			size:     size,
		}, nil
	}
	rac, err := openMMapFile(where, useMmapForLocalIndexes)
	if err != nil {
		return nil, fmt.Errorf("failed to open local index file: %w", err)
	}
	if !klog.V(5).Enabled() {
		return rac, nil
	}
	return &readCloserWrapper{
		rac:      rac,
		name:     where,
		isRemote: false,
	}, nil
}

func openMMapFile(filePath string, useMmap bool) (carreader.ReaderAtCloser, error) {
	if useMmap {
		return mmap.Open(filePath)
	}
	return os.Open(filePath)
}

func openCarStorage(
	ctx context.Context,
	where string,
	useMmap bool,
) (*carv2.Reader, carreader.ReaderAtCloser, error) {
	where = strings.TrimSpace(where)
	if isHTTP(where) {
		klog.Infof("opening CAR file from %q as HTTP remote file", where)
		rem, size, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(ctx, where)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open remote CAR file %q: %w", where, err)
		}
		return nil, &readCloserWrapper{
			rac:  rem,
			name: where,
			size: size,
		}, nil
	}

	if useMmap {
		carReader, err := carv2.OpenReader(where)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open CAR file: %w", err)
		}
		return carReader, nil, nil
	}
	reader, err := openCarFile(ctx, where)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open CAR file: %w", err)
	}
	return reader, nil, nil
}

func openCarFile(
	ctx context.Context,
	where string,
) (*carv2.Reader, error) {
	file, err := os.Open(where)
	if err != nil {
		return nil, fmt.Errorf("failed to open CAR file %q: %w", where, err)
	}
	r, err := carv2.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create CAR reader: %w", err)
	}
	return r, nil
}

func getTransactionAndMetaFromNode(
	transactionNode *ipldbindcode.Transaction,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) ([]byte, []byte, error) {
	transactionBuffer, err := ipldbindcode.LoadDataFromDataFrames(&transactionNode.Data, dataFrameGetter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load transaction: %w", err)
	}

	metaBuffer, err := ipldbindcode.LoadDataFromDataFrames(&transactionNode.Metadata, dataFrameGetter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load metadata: %w", err)
	}
	if len(metaBuffer) > 0 {
		uncompressedMeta, err := tooling.DecompressZstd(metaBuffer)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decompress metadata: %w", err)
		}
		return transactionBuffer, uncompressedMeta, nil
	}
	return transactionBuffer, nil, nil
}
