package nodetools

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ipld/go-car"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/schollz/progressbar/v3"
)

type BlockDAGs struct {
	reader   *carreader.PrefetchingCarReader
	callback func(*DataAndCidSlice) error
}

// NewBlockDag creates an iterator that reads a CAR file and presents to the provided callback function
// all nodes of a Block DAG as DataAndCidSlice.
func NewBlockDag(carPath string, callback func(*DataAndCidSlice) error) (*BlockDAGs, error) {
	file, err := os.Open(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CAR file: %w", err)
	}
	rd, err := carreader.NewPrefetching(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create prefetching car reader: %w", err)
	}
	return NewBlockDagFromReader(rd, callback), nil
}

func NewBlockDagWithProgressBar(
	carPath string,
	callback func(*DataAndCidSlice) error,
	progressBar *progressbar.ProgressBar,
) (*BlockDAGs, error) {
	file, err := os.Open(carPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CAR file: %w", err)
	}
	readerValue := progressbar.NewReader(file, progressBar)
	rd, err := carreader.NewPrefetching(io.NopCloser(&readerValue))
	if err != nil {
		return nil, fmt.Errorf("failed to create prefetching car reader: %w", err)
	}
	return NewBlockDagFromReader(rd, callback), nil
}

func NewBlockDagFromReader(reader *carreader.PrefetchingCarReader, callback func(*DataAndCidSlice) error) *BlockDAGs {
	return &BlockDAGs{
		reader:   reader,
		callback: callback,
	}
}

var ErrStopIteration = errors.New("stop iteration")

func (b *BlockDAGs) Header() *car.CarHeader {
	if b.reader == nil {
		return nil
	}
	return b.reader.Header
}

func (b *BlockDAGs) Do() error {
	if b.reader == nil {
		return errors.New("nil car reader")
	}
	if b.callback == nil {
		return errors.New("nil callback function")
	}

dagLoop:
	for {
		slice := getDataAndCidSlice()
		for {
			_cid, _, dataBuf, err := b.reader.NextNodeBytes()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil // End of file, exit the loop
				}
				return fmt.Errorf("failed to read next section: %w", err)
			}
			elem := &DataAndCid{
				Cid:  _cid,
				Data: dataBuf,
			}
			slice.Push(
				elem,
			)
			kind, err := iplddecoders.GetKind(dataBuf.Bytes())
			if err != nil {
				return fmt.Errorf("failed to get kind for CID %s: %w", _cid.String(), err)
			}
			if kind == iplddecoders.KindBlock {
				// If we encounter a Block, we process the slice and break out of the inner loop.
				if err := b.callback(slice); err != nil {
					if errors.Is(err, ErrStopIteration) {
						return nil // Stop iteration if the callback returns ErrStopIteration
					}
					// Handle any other error from the callback
					return fmt.Errorf("error processing block: %w", err)
				}
				continue dagLoop // We go to start the accumulating the next block DAG.
			}
		}
	}
}

func (b *BlockDAGs) Close() error {
	if b.reader == nil {
		return errors.New("nil car reader")
	}
	if err := b.reader.Close(); err != nil {
		return fmt.Errorf("failed to close car reader: %w", err)
	}
	return nil
}
