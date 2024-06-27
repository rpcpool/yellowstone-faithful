package linkedlog

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
	"github.com/mostynb/zstdpool-freelist"
)

var zstdDecoderPool = zstdpool.NewDecoderPool()

func decompressZSTD(data []byte) ([]byte, error) {
	dec, err := zstdDecoderPool.Get(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get zstd decoder from pool: %w", err)
	}
	defer zstdDecoderPool.Put(dec)

	content, err := dec.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress zstd data: %w", err)
	}
	return content, nil
}

var zstdEncoderPool = zstdpool.NewEncoderPool(
	zstd.WithEncoderLevel(zstd.SpeedBetterCompression),
	// zstd.WithEncoderLevel(zstd.SpeedFastest),
)

func compressZSTD(data []byte) ([]byte, error) {
	enc, err := zstdEncoderPool.Get(nil)
	if err != nil {
		return nil, err
	}
	defer zstdEncoderPool.Put(enc)
	return enc.EncodeAll(data, nil), nil
}
