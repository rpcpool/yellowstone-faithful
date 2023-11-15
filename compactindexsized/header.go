package compactindexsized

import (
	"bytes"
	"fmt"
	"io"
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

func (m *Meta) MarshalBinary() ([]byte, error) {
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

func (m *Meta) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	numKVs := int(b[0])
	if numKVs > MaxNumKVs {
		return fmt.Errorf("number of key-value pairs %d exceeds max %d", numKVs, MaxNumKVs)
	}
	b = b[1:]
	reader := bytes.NewReader(b)
	for i := 0; i < numKVs; i++ {
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

const (
	MaxNumKVs    = 255
	MaxKeySize   = 255
	MaxValueSize = 255
)

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
	m.KeyVals = append(m.KeyVals, KV{Key: key, Value: value})
	return nil
}

// GetFirst returns the first value for the given key.
func (m *Meta) GetFirst(key []byte) []byte {
	for _, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			return kv.Value
		}
	}
	return nil
}

// ReadFirst copies the first value for the given key into the given value.
// It returns the number of bytes copied.
func (m *Meta) ReadFirst(key []byte, valueDst []byte) int {
	for _, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			return copy(valueDst, kv.Value)
		}
	}
	return 0
}

// ReplaceFirst replaces the first value for the given key.
func (m *Meta) ReplaceFirst(key, value []byte) error {
	for i, kv := range m.KeyVals {
		if bytes.Equal(kv.Key, key) {
			m.KeyVals[i].Value = value
			return nil
		}
	}
	return fmt.Errorf("key %q not found", key)
}

// HasDuplicateKeys returns true if there are duplicate keys.
func (m *Meta) HasDuplicateKeys() bool {
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

// Get returns all values for the given key.
func (m *Meta) Get(key []byte) [][]byte {
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

var KeyKind = []byte{'k', 'i', 'n', 'd'}
