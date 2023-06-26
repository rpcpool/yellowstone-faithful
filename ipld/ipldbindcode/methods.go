package ipldbindcode

import (
	"fmt"
	"hash/crc64"
	"hash/fnv"
)

// DataFrame.HasHash returns whether the 'Hash' field is present.
func (n DataFrame) HasHash() bool {
	return n.Hash != nil && *n.Hash != nil
}

// GetHash returns the value of the 'Hash' field and
// a flag indicating whether the field has a value.
func (n DataFrame) GetHash() (int, bool) {
	if n.Hash == nil || *n.Hash == nil {
		return 0, false
	}
	return **n.Hash, true
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
func VerifyHash(data []byte, hash int) error {
	if checksumCrc64(data) != uint64(hash) {
		// Maybe it's the legacy checksum function?
		if checksumFnv(data) != uint64(hash) {
			return fmt.Errorf("data hash mismatch")
		}
	}
	return nil
}
