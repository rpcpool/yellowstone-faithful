package nodetools

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/util"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/valyala/bytebufferpool"
	"golang.org/x/sync/errgroup"
)

var dataAndCidSlicePool = &sync.Pool{
	New: func() any {
		return &DataAndCidSlice{}
	},
}

func getDataAndCidSlice() *DataAndCidSlice {
	got := dataAndCidSlicePool.Get().(*DataAndCidSlice)
	return got
}

func putDataAndCidSlice(slice *DataAndCidSlice) {
	slice.Reset()
	dataAndCidSlicePool.Put(slice)
}

// DataAndCid is a section that has been split into its CID and data components.
type DataAndCid struct {
	Cid  cid.Cid
	Data *bytebufferpool.ByteBuffer
}

type DataAndCidSlice []*DataAndCid

// DataAndCidSlice.GetByCid does a linear search for a CID in the DataAndCidSlice.
func (d DataAndCidSlice) GetByCid(c cid.Cid) (*DataAndCid, bool) {
	for i := range d {
		if d[i].Cid.Equals(c) {
			return d[i], true
		}
	}
	return nil, false
}

// DataAndCidSlice.SortByCid sorts the DataAndCidSlice by CID.
func (d DataAndCidSlice) SortByCid() {
	sort.Slice(d, func(i, j int) bool {
		return bytes.Compare(d[i].Cid.Bytes(), d[j].Cid.Bytes()) < 0
	})
}

func (d DataAndCidSlice) IsEmpty() bool {
	return len(d) == 0
}

// Push adds a DataAndCid to the DataAndCidSlice.
func (d *DataAndCidSlice) Push(dataAndCid *DataAndCid) {
	if d == nil {
		d = getDataAndCidSlice()
	}
	*d = append(*d, dataAndCid)
}

// method ReadEach(f func(d *DataAndCid) bool) {
func (d DataAndCidSlice) ReadEach(f func(d *DataAndCid) error) error {
	for i := range d {
		if err := f(d[i]); err != nil {
			return fmt.Errorf("error processing DataAndCid at index %d: %w", i, err)
		}
	}
	return nil
}

// NOTE: if you want to use this for parsing, use the serial version ReadEach instead.
// The cost of goroutines is higher than the cost of parsing each node serially.
func (d DataAndCidSlice) ReadEachConcurrent(f func(d *DataAndCid) error) error {
	if len(d) == 0 {
		return nil // nothing to do
	}
	wg := new(errgroup.Group)
	for i := range d {
		wg.Go(func() error {
			return f(d[i])
		})
	}
	return wg.Wait()
}

// DataAndCidSlice.ByCid does a binary search for a CID in the DataAndCidSlice.
// MUST be sorted by CID before calling this (using the SortByCid method).
// Returns the DataAndCid if found, or nil if not found.
func (d DataAndCidSlice) ByCid(c cid.Cid) (*DataAndCid, bool) {
	// do binary search for the CID
	i := sort.Search(len(d), func(i int) bool {
		return bytes.Compare(d[i].Cid.Bytes(), c.Bytes()) >= 0
	})
	if i < len(d) && d[i].Cid.Equals(c) {
		return d[i], true
	}
	return nil, false
}

func (d DataAndCidSlice) ToParsedAndCidSlice() (ParsedAndCidSlice, error) {
	parsed := make(ParsedAndCidSlice, len(d))
	for i := range d {
		node, err := iplddecoders.DecodeAny(d[i].Data.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode node with CID %s: %w", d[i].Cid, err)
		}
		pc := getParsedAndCid()
		pc.Cid = d[i].Cid
		pc.Data = node
		parsed[i] = pc
	}
	return parsed, nil
}

// *DataAndCidSlice.Reset resets the DataAndCidSlice to an empty slice.
func (d DataAndCidSlice) Reset() {
	for i := range d {
		if (d)[i] != nil {
			(d)[i].Put() // recycle each DataAndCid
			(d)[i] = nil // avoid memory leaks
		}
	}
	d = (d)[:0]
}

// Put recycles DataAndCidSlice, releasing it back to the pool.
func (d *DataAndCidSlice) Put() {
	putDataAndCidSlice(d) // recycle the DataAndCidSlice
}

var dataAndCidPool = &sync.Pool{
	New: func() any {
		return &DataAndCid{}
	},
}

func getDataAndCid() *DataAndCid {
	got := dataAndCidPool.Get().(*DataAndCid)
	if got.Data == nil {
		got.Data = bytebufferpool.Get()
	} else {
		got.Data.Reset()
	}
	return got
}

func putDataAndCid(d *DataAndCid) {
	d.Reset()
	dataAndCidPool.Put(d)
}

// DataAndCid.Reset resets the DataAndCid to its zero value.
func (d *DataAndCid) Reset() {
	if d == nil {
		return
	}
	d.Cid = cid.Undef // Reset the CID to avoid memory leaks.
	if d.Data != nil {
		bytebufferpool.Put(d.Data) // recycle the Data
		d.Data = nil               // Reset the Data to avoid memory leaks.
	}
}

func (d *DataAndCid) Put() {
	if d == nil {
		return
	}
	putDataAndCid(d) // recycle the DataAndCid
}

func SplitIntoDataAndCids(sections []byte) (DataAndCidSlice, error) {
	var nodes []*DataAndCid
	for len(sections) > 0 {
		gotLen, usize := binary.Uvarint(sections)
		if usize <= 0 {
			return nil, fmt.Errorf("failed to decode uvarint")
		}
		if gotLen > uint64(util.MaxAllowedSectionSize) { // Don't OOM
			return nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
		}
		cidLen, _cid, err := cid.CidFromReader(bytes.NewReader(sections[usize:]))
		if err != nil {
			return nil, fmt.Errorf("failed to read cid at %d element: %w", len(nodes), err)
		}
		dataStart := usize + cidLen
		dataEnd := int(gotLen) + usize

		node := getDataAndCid()
		node.Cid = _cid
		node.Data.Write(sections[dataStart:dataEnd])
		{
			if dataEnd > len(sections) {
				return nil, fmt.Errorf("dataEnd %d is out of bounds for data length %d", dataEnd, len(sections))
			}
		}
		nodes = append(nodes, node)
		sections = sections[dataEnd:]
	}
	return nodes, nil
}

func (d DataAndCidSlice) Blocks() ([]*ParsedAndCid, error) {
	blocks := make([]*ParsedAndCid, 0, len(d))
	for _, dataAndCid := range d {
		if dataAndCid == nil || dataAndCid.Data == nil {
			return nil, fmt.Errorf("nil DataAndCid or Data in DataAndCidSlice")
		}
		kind, err := iplddecoders.GetKind(dataAndCid.Data.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to get kind for CID %s: %w", dataAndCid.Cid, err)
		}
		if kind != iplddecoders.KindBlock {
			continue
		}
		block, err := iplddecoders.DecodeBlock(dataAndCid.Data.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode block with CID %s: %w", dataAndCid.Cid, err)
		}
		parsed := getParsedAndCid()
		parsed.Cid = dataAndCid.Cid
		parsed.Data = block
		blocks = append(blocks, parsed)
	}
	return blocks, nil
}
