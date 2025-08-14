//go:build !ffi
// +build !ffi

package jsonparsed

import (
	"encoding/json"
	"fmt"
)

func (inst Parameters) ParseInstruction() (json.RawMessage, error) {
	return nil, fmt.Errorf("not implemented")
}

// IsEnabled returns true if the library was build with the necessary
// flags to enable the FFI features necessary for parsing instructions.
func IsEnabled() bool {
	return false
}
