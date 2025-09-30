package ipldbindcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc64"
	"hash/fnv"
	"strconv"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/dummycid"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/tooling"
)

// DataFrame.HasHash returns whether the 'Hash' field is present.
func (n DataFrame) HasHash() bool {
	return n.Hash != nil && *n.Hash != nil
}

// GetHash returns the value of the 'Hash' field and
// a flag indicating whether the field has a value.
func (n DataFrame) GetHash() (uint64, bool) {
	if n.Hash == nil || *n.Hash == nil {
		return 0, false
	}
	return uint64(**n.Hash), true
}

// HasIndex returns whether the 'Index' field is present.
// If the field is not present, assume the index is 0.
func (n DataFrame) HasIndex() bool {
	return n.Index != nil && *n.Index != nil
}

// GetIndex returns the value of the 'Index' field and
// a flag indicating whether the field has a value.
// If the field is not present, assume the index is 0.
func (n DataFrame) GetIndex() (int, bool) {
	if n.Index == nil || *n.Index == nil {
		return 0, false
	}
	return **n.Index, true
}

// HasTotal returns whether the 'Total' field is present.
// If the field is not present, assume the total is 1.
func (n DataFrame) HasTotal() bool {
	return n.Total != nil && *n.Total != nil
}

// GetTotal returns the value of the 'Total' field and
// a flag indicating whether the field has a value.
// If the field is not present, assume the total is 1.
func (n DataFrame) GetTotal() (int, bool) {
	if n.Total == nil || *n.Total == nil {
		return 0, false
	}
	return **n.Total, true
}

// GetData returns the value of the 'Data' field and
// a flag indicating whether the field has a value.
func (n DataFrame) Bytes() []uint8 {
	return n.Data
}

// HasNext returns whether the 'Next' field is present and non-empty.
func (n DataFrame) HasNext() bool {
	return n.Next != nil && *n.Next != nil && len(**n.Next) > 0
}

// GetNext returns the value of the 'Next' field and
// a flag indicating whether the field has a value.
func (n DataFrame) GetNext() (List__Link, bool) {
	if n.Next == nil || *n.Next == nil || **n.Next == nil {
		return nil, false
	}
	return **n.Next, true
}

// checksumFnv is the legacy checksum function, used in the first version of the radiance
// car creator. Some old cars still use this function.
func checksumFnv(data []byte) uint64 {
	h := fnv.New64a()
	h.Write(data)
	return h.Sum64()
}

// checksumCrc64 returns the hash of the provided buffer.
// It is used in the latest version of the radiance car creator.
func checksumCrc64(buf []byte) uint64 {
	return crc64.Checksum(buf, crc64.MakeTable(crc64.ISO))
}

// VerifyHash verifies that the provided data matches the provided hash.
// In case of DataFrames, the hash is stored in the 'Hash' field, and
// it is the hash of the concatenated 'Data' fields of all the DataFrames.
func VerifyHash(data []byte, hash uint64) error {
	if checksumCrc64(data) != (hash) {
		// Maybe it's the legacy checksum function?
		if checksumFnv(data) != (hash) {
			return fmt.Errorf("data hash mismatch")
		}
	}
	return nil
}

// Transaction.HasIndex returns whether the 'Index' field is present.
func (n Transaction) HasIndex() bool {
	return n.Index != nil && *n.Index != nil
}

// GetPositionIndex returns the 'Index' field, which indicates
// the index of the transaction in the block (0-based), and
// a flag indicating whether the field has a value.
func (n Transaction) GetPositionIndex() (int, bool) {
	if n.Index == nil || *n.Index == nil {
		return 0, false
	}
	return **n.Index, true
}

var DisableHashVerification bool

func (decoded *Transaction) GetSolanaTransaction() (*solana.Transaction, error) {
	if total, ok := decoded.Data.GetTotal(); !ok || total == 1 {
		completeData := decoded.Data.Bytes()
		if !DisableHashVerification {
			// verify hash (if present)
			if ha, ok := decoded.Data.GetHash(); ok {
				err := VerifyHash(completeData, ha)
				if err != nil {
					return nil, fmt.Errorf("error while verifying hash: %w", err)
				}
			}
		}
		var tx solana.Transaction
		if err := bin.UnmarshalBin(&tx, completeData); err != nil {
			return nil, fmt.Errorf("error while unmarshaling transaction: %w", err)
		} else if len(tx.Signatures) == 0 {
			return nil, fmt.Errorf("transaction has no signatures")
		}
		return &tx, nil
	} else {
		return nil, errors.New("transaction data is split into multiple objects")
	}
}

var (
	ErrPiecesNotAvailable = errors.New("transaction pieces are not available")
	ErrMetadataNotFound   = errors.New("transaction metadata not found")
)

// GetMetadata will parse and return the metadata of the transaction.
// NOTE: This will return ErrPiecesNotAvailable if the metadata is split into multiple dataframes.
// In that case, you should use GetMetadataWithFrameLoader instead.
func (decodedTxObj *Transaction) GetMetadata() (*solanatxmetaparsers.TransactionStatusMetaContainer, error) {
	if total, ok := decodedTxObj.Metadata.GetTotal(); !ok || total == 1 {
		// metadata fit into the transaction object:
		completeBuffer := decodedTxObj.Metadata.Bytes()
		if ha, ok := decodedTxObj.Metadata.GetHash(); ok {
			err := VerifyHash(completeBuffer, ha)
			if err != nil {
				return nil, fmt.Errorf("failed to verify metadata hash: %w", err)
			}
		}
		if len(completeBuffer) > 0 {
			uncompressedMeta, err := tooling.DecompressZstd(completeBuffer)
			if err != nil {
				return nil, fmt.Errorf("failed to decompress metadata: %w", err)
			}
			status, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(uncompressedMeta)
			if err == nil {
				return status, nil
			} else {
				return nil, fmt.Errorf("failed to parse metadata: %w", err)
			}
		} else {
			return nil, ErrMetadataNotFound
		}
	}
	// metadata didn't fit into the transaction object, and was split into multiple dataframes.
	return nil, ErrPiecesNotAvailable
}

// GetMetadataWithFrameLoader will parse and return the metadata of the transaction.
// It uses the provided dataFrameGetter function to load the missing dataframes.
func (decodedTxObj *Transaction) GetMetadataWithFrameLoader(dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*DataFrame, error)) (*solanatxmetaparsers.TransactionStatusMetaContainer, error) {
	if total, ok := decodedTxObj.Metadata.GetTotal(); !ok || total == 1 {
		return decodedTxObj.GetMetadata()
	}
	// metadata didn't fit into the transaction object, and was split into multiple dataframes:
	metaBuffer, err := LoadDataFromDataFrames(
		&decodedTxObj.Metadata,
		dataFrameGetter,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	// if we have a metadata buffer, try to decompress it:
	if len(metaBuffer) > 0 {
		uncompressedMeta, err := tooling.DecompressZstd(metaBuffer)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress metadata: %w", err)
		}
		status, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(uncompressedMeta)
		if err == nil {
			return status, nil
		} else {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
	} else {
		return nil, ErrMetadataNotFound
	}
}

func (decoded *Transaction) Signatures() ([]solana.Signature, error) {
	return readAllSignatures(decoded.Data.Bytes())
}

func (decoded *Transaction) Signature() (solana.Signature, error) {
	return tooling.ReadFirstSignature(decoded.Data.Bytes())
}

func readAllSignatures(buf []byte) ([]solana.Signature, error) {
	decoder := bin.NewCompactU16Decoder(buf)
	numSigs, err := decoder.ReadCompactU16()
	if err != nil {
		return nil, err
	}
	if numSigs == 0 {
		return nil, fmt.Errorf("no signatures")
	}
	// check that there is at least 64 bytes * numSigs left:
	if decoder.Remaining() < (64 * numSigs) {
		return nil, fmt.Errorf("not enough bytes left to read %d signatures", numSigs)
	}

	sigs := make([]solana.Signature, numSigs)
	for i := 0; i < numSigs; i++ {
		numRead, err := decoder.Read(sigs[i][:])
		if err != nil {
			return nil, err
		}
		if numRead != 64 {
			return nil, fmt.Errorf("unexpected signature length %d", numRead)
		}
	}
	return sigs, nil
}

// GetBlockHeight returns the 'block_height' field, which indicates
// the height of the block, and
// a flag indicating whether the field has a value.
func (n Block) GetBlockHeight() (uint64, bool) {
	if n.Meta.Block_height == nil || *n.Meta.Block_height == nil {
		return 0, false
	}
	return uint64(**n.Meta.Block_height), true
}

func (n Block) GetRewards() (cid.Cid, bool) {
	rewardsCid := n.Rewards.(cidlink.Link).Cid
	if rewardsCid.Equals(dummycid.DummyCID) {
		return cid.Cid{}, false
	}
	return rewardsCid, true
}

// DataFrame.MarshalJSON implements the json.Marshaler interface.
func (n DataFrame) MarshalJSON() ([]byte, error) {
	out := new(strings.Builder)
	out.WriteString(`{"kind":`)
	out.WriteString(fmt.Sprintf("%d", n.Kind))
	if n.Hash != nil && *n.Hash != nil {
		out.WriteString(`,"hash":`)
		out.WriteString(fmt.Sprintf(`"%d"`, uint64(**n.Hash)))
	} else {
		out.WriteString(`,"hash":null`)
	}

	if n.Index != nil && *n.Index != nil {
		out.WriteString(`,"index":`)
		out.WriteString(fmt.Sprintf("%d", **n.Index))
	} else {
		out.WriteString(`,"index":null`)
	}
	if n.Total != nil && *n.Total != nil {
		out.WriteString(`,"total":`)
		out.WriteString(fmt.Sprintf("%d", **n.Total))
	} else {
		out.WriteString(`,"total":null`)
	}
	out.WriteString(`,"data":`)
	out.WriteString(fmt.Sprintf("%q", n.Data.String()))
	if n.Next != nil && *n.Next != nil {
		out.WriteString(`,"next":`)
		nextAsJSON, err := json.Marshal(**n.Next)
		if err != nil {
			return nil, err
		}
		out.Write(nextAsJSON)
	} else {
		out.WriteString(`,"next":null`)
	}
	out.WriteString("}")
	return []byte(out.String()), nil
}

// DataFrame.UnmarshalJSON implements the json.Unmarshaler interface.
func (n *DataFrame) UnmarshalJSON(data []byte) error {
	// We have to use a custom unmarshaler because we need to
	// unmarshal the 'data' field as a string, and then convert
	// it to a byte slice.
	type Alias DataFrame

	type CidObj map[string]string
	aux := &struct {
		Data string   `json:"data"`
		Hash string   `json:"hash"`
		Next []CidObj `json:"next"`
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if err := n.Data.FromString(aux.Data); err != nil {
		return err
	}
	if aux.Hash != "" {
		hash, err := strconv.ParseUint(aux.Hash, 10, 64)
		if err != nil {
			return err
		}
		h := int(hash)
		hp := &h
		n.Hash = &hp
	}
	if len(aux.Next) > 0 {
		next := List__Link{}
		for _, c := range aux.Next {
			decoded, err := cid.Decode(c["/"])
			if err != nil {
				return err
			}
			next = append(next, cidlink.Link{Cid: decoded})
		}
		nextP := &next
		n.Next = &nextP
	}
	return nil
}

// SlotMeta.HasBlockHeight returns whether the 'Block_height' field is present.
func (n SlotMeta) HasBlockHeight() bool {
	return n.Block_height != nil && *n.Block_height != nil
}

// GetBlockHeight returns the value of the 'Block_height' field and
// a flag indicating whether the field has a value.
func (n SlotMeta) GetBlockHeight() (uint64, bool) {
	if n.Block_height == nil || *n.Block_height == nil {
		return 0, false
	}
	return uint64(**n.Block_height), true
}

// SlotMeta.Equivalent returns whether the two SlotMeta objects are equivalent.
func (n SlotMeta) Equivalent(other SlotMeta) bool {
	if n.Parent_slot != other.Parent_slot {
		return false
	}
	if n.Blocktime != other.Blocktime {
		return false
	}
	bh1, ok1 := n.GetBlockHeight()
	bh2, ok2 := other.GetBlockHeight()
	if ok1 != ok2 {
		return false
	}
	if ok1 && bh1 != bh2 {
		return false
	}
	return true
}

// Block.HasRewards
func (n Block) HasRewards() bool {
	hasRewards := !n.Rewards.(cidlink.Link).Cid.Equals(dummycid.DummyCID)
	return hasRewards
}

// Reset resets the List__Link to an empty state.
func (l *List__Link) Reset() {
	if l == nil {
		return
	}
	*l = (*l)[:0] // Reset the slice to an empty state.
}

// Reset resets the Epoch to an empty state.
func (e *Epoch) Reset() {
	if e == nil {
		return
	}
	e.Kind = 0
	e.Epoch = 0
	e.Subsets.Reset() // Reset the slice to an empty state.
}

// Reset resets the Subset to an empty state.
func (s *Subset) Reset() {
	if s == nil {
		return
	}
	s.Kind = 0
	s.First = 0
	s.Last = 0
	s.Blocks.Reset() // Reset the slice to an empty state.
}

// Reset resets the List__Shredding to an empty state.
func (l *List__Shredding) Reset() {
	if l == nil {
		return
	}
	*l = (*l)[:0] // Reset the slice to an empty state.
}

// Reset resets the Block to an empty state.
func (b *Block) Reset() {
	if b == nil {
		return
	}
	b.Kind = 0
	b.Slot = 0
	b.Shredding.Reset()                              // Reset the slice to an empty state.
	b.Entries.Reset()                                // Reset the slice to an empty state.
	b.Meta = SlotMeta{}                              // Reset the SlotMeta to an empty state.
	b.Rewards = cidlink.Link{Cid: dummycid.DummyCID} // Reset the Rewards to a dummy CID.
}

// Reset resets the Rewards to an empty state.
func (r *Rewards) Reset() {
	if r == nil {
		return
	}
	r.Kind = 0
	r.Slot = 0
	r.Data.Reset() // Reset the DataFrame to an empty state.
}

// Reset resets the SlotMeta to an empty state.
func (s *SlotMeta) Reset() {
	if s == nil {
		return
	}
	s.Parent_slot = 0
	s.Blocktime = 0
	clearIntptrPtr(s.Block_height) // Reset the Block_height pointer to nil.
	s.Block_height = nil           // Reset the pointer to nil.
}

// Reset resets the Shredding to an empty state.
func (s *Shredding) Reset() {
	if s == nil {
		return
	}
	s.EntryEndIdx = 0
	s.ShredEndIdx = 0
}

// Reset resets the Entry to an empty state.
func (e *Entry) Reset() {
	if e == nil {
		return
	}
	e.Kind = 0
	e.NumHashes = 0
	e.Hash = e.Hash[:0]    // Reset the Hash to an empty slice.
	e.Transactions.Reset() // Reset the slice to an empty state.
}

// Reset resets the DataFrame to an empty state.
func (d *DataFrame) Reset() {
	if d == nil {
		return
	}
	d.Kind = 0
	if d.Hash != nil {
		*d.Hash = nil // Reset the pointer to nil.
	}
	d.Hash = nil            // Reset the pointer to nil.
	clearIntptrPtr(d.Index) // Reset the Index pointer to nil.
	clearIntptrPtr(d.Total) // Reset the Total pointer to nil.
	d.Total = nil           // Reset the pointer to nil.
	d.Data = d.Data[:0]     // Reset the Data slice to an empty state.
	if d.Next != nil && *d.Next != nil {
		(*d.Next).Reset() // Reset the List__Link to an empty state.
	} else {
		d.Next = nil // Reset the pointer to nil.
	}
}

// Reset resets the Transaction to an empty state.
func (t *Transaction) Reset() {
	if t == nil {
		return
	}
	t.Kind = 0
	t.Data.Reset() // Reset the DataFrame to an empty state.
	t.Metadata.Reset()
	t.Slot = 0
	clearIntptrPtr(t.Index) // Reset the Index pointer to nil.
}

func clearIntptrPtr(ptr **int) {
	if ptr == nil || *ptr == nil {
		return
	}
	**ptr = 0  // Reset the value to 0.
	*ptr = nil // Reset the pointer to nil.
}

type Node interface {
	Node()
}

var (
	_ Node = Epoch{}
	_ Node = Subset{}
	_ Node = Block{}
	_ Node = Rewards{}
	_ Node = Entry{}
	_ Node = Transaction{}
	_ Node = DataFrame{}
)

func (e Epoch) Node() {}

func (s Subset) Node() {}

func (b Block) Node() {}

func (r Rewards) Node() {}

func (e Entry) Node() {}

func (t Transaction) Node() {}

func (d DataFrame) Node() {}

// GetSlot returns the slot of the block.
func (b *Block) GetSlot() uint64 {
	if b == nil {
		return 0
	}
	return uint64(b.Slot)
}

// GetBlocktime returns the blocktime of the block.
func (m *SlotMeta) GetBlocktime() int64 {
	if m == nil {
		return 0
	}
	return int64(m.Blocktime)
}

func (b *Block) GetParentSlot() uint64 {
	if b == nil || b.Meta.Parent_slot == 0 {
		return 0
	}
	return uint64(b.Meta.Parent_slot)
}

func (b *Block) GetBlocktime() int64 {
	if b == nil || b.Meta.Blocktime == 0 {
		return 0
	}
	return int64(b.Meta.Blocktime)
}
