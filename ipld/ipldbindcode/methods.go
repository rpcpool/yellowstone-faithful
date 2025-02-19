package ipldbindcode

import (
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
func (n DataFrame) HasIndex() bool {
	return n.Index != nil && *n.Index != nil
}

// GetIndex returns the value of the 'Index' field and
// a flag indicating whether the field has a value.
func (n DataFrame) GetIndex() (int, bool) {
	if n.Index == nil || *n.Index == nil {
		return 0, false
	}
	return **n.Index, true
}

// HasTotal returns whether the 'Total' field is present.
func (n DataFrame) HasTotal() bool {
	return n.Total != nil && *n.Total != nil
}

// GetTotal returns the value of the 'Total' field and
// a flag indicating whether the field has a value.
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
	if n.Next == nil || *n.Next == nil {
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

func (decoded Transaction) GetSolanaTransaction() (*solana.Transaction, error) {
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

func (decoded Transaction) Signatures() ([]solana.Signature, error) {
	return readAllSignatures(decoded.Data.Bytes())
}

func (decoded Transaction) Signature() (solana.Signature, error) {
	return readFirstSignature(decoded.Data.Bytes())
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

func readFirstSignature(buf []byte) (solana.Signature, error) {
	decoder := bin.NewCompactU16Decoder(buf)
	numSigs, err := decoder.ReadCompactU16()
	if err != nil {
		return solana.Signature{}, err
	}
	if numSigs == 0 {
		return solana.Signature{}, fmt.Errorf("no signatures")
	}
	// check that there is at least 64 bytes left:
	if decoder.Remaining() < 64 {
		return solana.Signature{}, fmt.Errorf("not enough bytes left to read a signature")
	}

	var sig solana.Signature
	numRead, err := decoder.Read(sig[:])
	if err != nil {
		return sig, err
	}
	if numRead != 64 {
		return sig, fmt.Errorf("unexpected signature length %d", numRead)
	}
	return sig, nil
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
	n.Data.FromString(aux.Data)
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
