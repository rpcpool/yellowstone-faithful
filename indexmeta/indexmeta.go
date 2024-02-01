package indexmeta

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	bin "github.com/gagliardetto/binary"
	"github.com/ipfs/go-cid"
)

const (
	MaxNumKVs    = 255
	MaxKeySize   = 255
	MaxValueSize = 255
)

type Meta struct {
	KeyVals []KV
}

// Bytes returns the serialized metadata.
func (m *Meta) Bytes() []byte {
	b, err := m.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return b
}

func (m Meta) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	if len(m.KeyVals) > MaxNumKVs {
		return nil, fmt.Errorf("number of key-value pairs %d exceeds max %d", len(m.KeyVals), MaxNumKVs)
	}
	buf.WriteByte(byte(len(m.KeyVals)))
	for i, kv := range m.KeyVals {
		{
			keyLen := len(kv.Key)
			if keyLen > MaxKeySize {
				return nil, fmt.Errorf("key %d size %d exceeds max %d", i, keyLen, MaxKeySize)
			}
			buf.WriteByte(byte(keyLen))
			buf.Write(kv.Key)
		}
		{
			valueLen := len(kv.Value)
			if valueLen > MaxValueSize {
				return nil, fmt.Errorf("value %d size %d exceeds max %d", i, valueLen, MaxValueSize)
			}
			buf.WriteByte(byte(valueLen))
			buf.Write(kv.Value)
		}
	}
	return buf.Bytes(), nil
}

type Decoder interface {
	io.ByteReader
	io.Reader
}

func (m *Meta) UnmarshalWithDecoder(decoder Decoder) error {
	numKVs, err := decoder.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read number of key-value pairs: %w", err)
	}
	if numKVs > MaxNumKVs {
		return fmt.Errorf("number of key-value pairs %d exceeds max %d", numKVs, MaxNumKVs)
	}
	reader := decoder
	for i := 0; i < int(numKVs); i++ {
		var kv KV
		{
			keyLen, err := reader.ReadByte()
			if err != nil {
				return fmt.Errorf("failed to read key length %d: %w", i, err)
			}
			kv.Key = make([]byte, keyLen)
			if _, err := io.ReadFull(reader, kv.Key); err != nil {
				return fmt.Errorf("failed to read key %d: %w", i, err)
			}
		}
		{
			valueLen, err := reader.ReadByte()
			if err != nil {
				return fmt.Errorf("failed to read value length %d: %w", i, err)
			}
			kv.Value = make([]byte, valueLen)
			if _, err := io.ReadFull(reader, kv.Value); err != nil {
				return fmt.Errorf("failed to read value %d: %w", i, err)
			}
		}
		m.KeyVals = append(m.KeyVals, kv)
	}
	return nil
}

func (m *Meta) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	decoder := bin.NewBorshDecoder(b)
	return m.UnmarshalWithDecoder(decoder)
}

// Add adds a key-value pair to the metadata.
func (m *Meta) Add(key, value []byte) error {
	if len(m.KeyVals) >= MaxNumKVs {
		return fmt.Errorf("number of key-value pairs %d exceeds max %d", len(m.KeyVals), MaxNumKVs)
	}
	if len(key) > MaxKeySize {
		return fmt.Errorf("key size %d exceeds max %d", len(key), MaxKeySize)
	}
	if len(value) > MaxValueSize {
		return fmt.Errorf("value size %d exceeds max %d", len(value), MaxValueSize)
	}
	m.KeyVals = append(m.KeyVals, KV{Key: cloneBytes(key), Value: cloneBytes(value)})
	return nil
}

func cloneBytes(b []byte) []byte {
	return append([]byte(nil), b...)
}

func (m *Meta) AddCid(key []byte, value cid.Cid) error {
	return m.Add(key, value.Bytes())
}

func (m Meta) GetCid(key []byte) (cid.Cid, bool) {
	value, ok := m.Get(key)
	if !ok {
		return cid.Undef, false
	}
	_, _cid, err := cid.CidFromBytes(value)
	if err != nil {
		return cid.Undef, false
	}
	return _cid, true
}

func (m *Meta) AddString(key []byte, value string) error {
	return m.Add(key, []byte(value))
}

func (m Meta) GetString(key []byte) (string, bool) {
	value, ok := m.Get(key)
	if !ok {
		return "", false
	}
	return string(value), true
}

func (m *Meta) AddUint64(key []byte, value uint64) error {
	return m.Add(key, encodeUint64(value))
}

func (m Meta) GetUint64(key []byte) (uint64, bool) {
	value, ok := m.Get(key)
	if !ok {
		return 0, false
	}
	return decodeUint64(value), true
}

func encodeUint64(value uint64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, value)
	return buf
}

func decodeUint64(buf []byte) uint64 {
	return binary.LittleEndian.Uint64(buf)
}

// Replace replaces the first value for the given key.
func (m *Meta) Replace(key, value []byte) error {
	if len(m.KeyVals) >= MaxNumKVs {
		return fmt.Errorf("number of key-value pairs %d exceeds max %d", len(m.KeyVals), MaxNumKVs)
	}
	if len(key) > MaxKeySize {
		return fmt.Errorf("key size %d exceeds max %d", len(key), MaxKeySize)
	}
	if len(value) > MaxValueSize {
		return fmt.Errorf("value size %d exceeds max %d", len(value), MaxValueSize)
	}
	for i, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			m.KeyVals[i].Value = cloneBytes(value)
			return nil
		}
	}
	return fmt.Errorf("key %q not found", key)
}

// Get returns the first value for the given key.
func (m Meta) Get(key []byte) ([]byte, bool) {
	for _, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			return kv.Value, true
		}
	}
	return nil, false
}

// ReadFirst copies the first value for the given key into the given value.
// It returns the number of bytes copied.
func (m Meta) ReadFirst(key []byte, valueDst []byte) int {
	for _, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			return copy(valueDst, kv.Value)
		}
	}
	return 0
}

// HasDuplicateKeys returns true if there are duplicate keys.
func (m Meta) HasDuplicateKeys() bool {
	seen := make(map[string]struct{})
	for _, kv := range m.KeyVals {
		if _, ok := seen[string(kv.Key)]; ok {
			return true
		}
		seen[string(kv.Key)] = struct{}{}
	}
	return false
}

func (m *Meta) Remove(key []byte) {
	var newKeyVals []KV
	for _, kv := range m.KeyVals {
		if !bytes.Equal(kv.Key, key) {
			newKeyVals = append(newKeyVals, kv)
		}
	}
	m.KeyVals = newKeyVals
}

// GetAll returns all values for the given key.
func (m Meta) GetAll(key []byte) [][]byte {
	var values [][]byte
	for _, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			values = append(values, kv.Value)
		}
	}
	return values
}

// Count returns the number of values for the given key.
func (m *Meta) Count(key []byte) int {
	var count int
	for _, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			count++
		}
	}
	return count
}

type KV struct {
	Key   []byte
	Value []byte
}

func NewKV(key, value []byte) KV {
	return KV{Key: key, Value: value}
}
