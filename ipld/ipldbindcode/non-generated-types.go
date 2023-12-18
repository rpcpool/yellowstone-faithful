package ipldbindcode

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
)

type (
	Hash   []uint8
	Buffer []uint8
)

// Hash.String() returns the string representation of the Hash in hex.
func (h Hash) String() string {
	return hex.EncodeToString(h)
}

// Buffer.String() returns the string representation of the Buffer in base64.
func (b Buffer) String() string {
	return base64.StdEncoding.EncodeToString(b)
}

func (b *Buffer) FromString(s string) error {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	*b = decoded
	return nil
}

// Buffer.MarshalJSON() returns the JSON representation of the Buffer in base64.
func (b Buffer) MarshalJSON() ([]byte, error) {
	return []byte("\"" + b.String() + "\""), nil
}

// Buffer.UnmarshalJSON() decodes the JSON representation of the Buffer in base64.
func (b *Buffer) UnmarshalJSON(data []byte) error {
	// strip the quotes
	dataAsString, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	// decode the base64
	decoded, err := base64.StdEncoding.DecodeString(dataAsString)
	if err != nil {
		return err
	}
	*b = decoded
	return nil
}

// Hash.MarshalJSON() returns the JSON representation of the Hash in hex.
func (h Hash) MarshalJSON() ([]byte, error) {
	return []byte("\"" + h.String() + "\""), nil
}

// Hash.UnmarshalJSON() decodes the JSON representation of the Hash in hex.
func (h *Hash) UnmarshalJSON(data []byte) error {
	// strip the quotes
	dataAsString, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	// decode the hex
	decoded, err := hex.DecodeString(dataAsString)
	if err != nil {
		return err
	}
	*h = decoded
	return nil
}
