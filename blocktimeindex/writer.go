package blocktimeindex

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/indexes"
	"github.com/rpcpool/yellowstone-faithful/slottools"
)

// Using an int32 for blocktime is enough seconds until the year 2106.

var magic = []byte("blocktimeindex")

type Index struct {
	start    uint64
	end      uint64
	epoch    uint64
	capacity uint64
	values   []int64
}

func NewIndexer(start, end, capacity uint64) *Index {
	epoch := slottools.EpochForSlot(start)
	if epoch != slottools.EpochForSlot(end) {
		panic(fmt.Sprintf("start and end slots must be in the same epoch: %d != %d", epoch, slottools.EpochForSlot(end)))
	}
	return &Index{
		start:    start,
		end:      end,
		epoch:    epoch,
		capacity: capacity,
		values:   make([]int64, capacity),
	}
}

const DefaultCapacityForEpoch = 432_000

// NewForEpoch creates a new Index for the given epoch.
func NewForEpoch(epoch uint64) *Index {
	start, end := slottools.CalcEpochLimits(epoch)
	return NewIndexer(start, end, DefaultCapacityForEpoch)
}

// Set sets the blocktime for the given slot.
func (i *Index) Set(slot uint64, time int64) error {
	if slot < i.start || slot > i.end {
		return NewErrSlotOutOfRange(i.start, i.end, slot)
	}
	i.values[slot-i.start] = time
	return nil
}

// Get gets the blocktime for the given slot.
func (i *Index) Get(slot uint64) (int64, error) {
	if slot < i.start || slot > i.end {
		return 0, NewErrSlotOutOfRange(i.start, i.end, slot)
	}
	return i.values[slot-i.start], nil
}

func (i *Index) marshalBinary() ([]byte, error) {
	writer := bytes.NewBuffer(nil)
	writer.Grow(DefaultIndexByteSize)
	_, err := writer.Write(magic)
	if err != nil {
		return nil, fmt.Errorf("failed to write magic: %w", err)
	}
	_, err = writer.Write(slottools.Uint64ToLEBytes(i.start))
	if err != nil {
		return nil, fmt.Errorf("failed to write start: %w", err)
	}
	_, err = writer.Write(slottools.Uint64ToLEBytes(i.end))
	if err != nil {
		return nil, fmt.Errorf("failed to write end: %w", err)
	}
	_, err = writer.Write(slottools.Uint64ToLEBytes(i.epoch))
	if err != nil {
		return nil, fmt.Errorf("failed to write epoch: %w", err)
	}
	_, err = writer.Write(slottools.Uint64ToLEBytes(i.capacity))
	if err != nil {
		return nil, fmt.Errorf("failed to write capacity: %w", err)
	}
	for _, t := range i.values {
		b, err := blocktimeToBytes(int64(t))
		if err != nil {
			return nil, fmt.Errorf("failed to convert time to bytes: %w", err)
		}
		_, err = writer.Write(b)
		if err != nil {
			return nil, fmt.Errorf("failed to write time: %w", err)
		}
	}
	return writer.Bytes(), nil
}

func (i *Index) MarshalBinary() ([]byte, error) {
	return i.marshalBinary()
}

var _ io.WriterTo = (*Index)(nil)

// WriteTo implements io.WriterTo.
func (i *Index) WriteTo(wr io.Writer) (int64, error) {
	data, err := i.marshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := wr.Write(data)
	return int64(n), err
}

func blocktimeToBytes(blocktime int64) ([]byte, error) {
	if blocktime < 0 {
		return nil, fmt.Errorf("blocktime must be non-negative")
	}
	if blocktime > math.MaxUint32 {
		return nil, fmt.Errorf("blocktime must fit in 32 bits")
	}
	// treat as uint32
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(blocktime))
	return buf, nil
}

func (i *Index) unmarshalBinary(data []byte) error {
	reader := bytes.NewReader(data)
	magicBuf := make([]byte, len(magic))
	_, err := reader.Read(magicBuf)
	if err != nil {
		return fmt.Errorf("failed to read magic: %w", err)
	}
	if !bytes.Equal(magicBuf, magic) {
		return fmt.Errorf("invalid magic: %s", magicBuf)
	}

	startBuf := make([]byte, 8)
	_, err = reader.Read(startBuf)
	if err != nil {
		return fmt.Errorf("failed to read start: %w", err)
	}
	i.start = slottools.Uint64FromLEBytes(startBuf)

	endBuf := make([]byte, 8)
	_, err = reader.Read(endBuf)
	if err != nil {
		return fmt.Errorf("failed to read end: %w", err)
	}
	i.end = slottools.Uint64FromLEBytes(endBuf)

	epochBuf := make([]byte, 8)
	_, err = reader.Read(epochBuf)
	if err != nil {
		return fmt.Errorf("failed to read epoch: %w", err)
	}
	i.epoch = slottools.Uint64FromLEBytes(epochBuf)
	{
		// check that start and end are in the same epoch
		startEpoch := slottools.EpochForSlot(i.start)
		endEpoch := slottools.EpochForSlot(i.end)
		if startEpoch != endEpoch {
			return fmt.Errorf("start and end slots must be in the same epoch: %d != %d", startEpoch, endEpoch)
		}
		if startEpoch != i.epoch {
			return fmt.Errorf("epoch mismatch: start=%d, end=%d, epoch=%d", startEpoch, endEpoch, i.epoch)
		}
	}

	capacityBuf := make([]byte, 8)
	_, err = reader.Read(capacityBuf)
	if err != nil {
		return fmt.Errorf("failed to read capacity: %w", err)
	}
	i.capacity = slottools.Uint64FromLEBytes(capacityBuf)

	i.values = make([]int64, i.capacity)
	for j := uint64(0); j < i.capacity; j++ {
		timeBuf := make([]byte, 4)
		_, err = reader.Read(timeBuf)
		if err != nil {
			return fmt.Errorf("failed to read time: %w", err)
		}
		i.values[j] = int64(binary.LittleEndian.Uint32(timeBuf))
	}
	return nil
}

func (i *Index) UnmarshalBinary(data []byte) error {
	return i.unmarshalBinary(data)
}

func (i *Index) FromBytes(data []byte) error {
	return i.unmarshalBinary(data)
}

func (i *Index) FromReader(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}
	return i.FromBytes(data)
}

func (i *Index) FromFile(file string) error {
	buf, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	return i.FromBytes(buf)
}

func FromFile(file string) (*Index, error) {
	i := &Index{}
	err := i.FromFile(file)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func FromBytes(data []byte) (*Index, error) {
	i := &Index{}
	err := i.FromBytes(data)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func FromReader(r io.Reader) (*Index, error) {
	i := &Index{}
	err := i.FromReader(r)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func FormatFilename(epoch uint64, rootCid cid.Cid, network indexes.Network) string {
	return fmt.Sprintf(
		"epoch-%d-%s-%s-%s",
		epoch,
		rootCid.String(),
		network,
		"slot-to-blocktime.index",
	)
}

var DefaultIndexByteSize = len(magic) + 8 + 8 + 8 + 8 + (432000 * 4)

func (i *Index) Epoch() uint64 {
	return i.epoch
}
