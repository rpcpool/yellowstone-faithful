//go:build ffi
// +build ffi

package jsonparsed

/*
#cgo LDFLAGS: -L./lib -lsolana_transaction_status_wrapper -lm -ldl
#include <stdlib.h>
#include "./lib/transaction_status.h"

typedef unsigned char u_char;
*/
import "C"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unsafe"

	bin "github.com/gagliardetto/binary"
)

func (inst Parameters) ParseInstruction() (json.RawMessage, error) {
	buf := new(bytes.Buffer)
	buf.Grow(1024)
	encoder := bin.NewBinEncoder(buf)

	err := inst.MarshalWithEncoder(encoder)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Parameters: %w", err)
	}

	cs := (*C.u_char)(C.CBytes(buf.Bytes()))
	defer C.free(unsafe.Pointer(cs))

	got := C.parse_instruction(cs, C.ulong(len(buf.Bytes())))

	parsedInstructionJSON := C.GoBytes(unsafe.Pointer(got.buf.data), C.int(got.buf.len))
	// Free the Rust-allocated memory
	C.free_response(got.buf.data, C.ulong(got.buf.len))

	// Check if the response is all zeros
	allZeros := true
	for _, b := range parsedInstructionJSON {
		if b != 0 {
			allZeros = false
			break
		}
	}

	if allZeros {
		fmt.Printf("[golang] WARNING: Response is all zeros for program %s\n", inst.ProgramID.String())
		// Return nil to trigger the fallback to unparsed format
		return nil, fmt.Errorf("parser returned empty response")
	}

	// Check if it's valid JSON
	jsonStr := string(parsedInstructionJSON)
	if !strings.HasPrefix(strings.TrimSpace(jsonStr), "{") && !strings.HasPrefix(strings.TrimSpace(jsonStr), "[") {
		return nil, fmt.Errorf("parser returned non-JSON response")
	}

	return parsedInstructionJSON, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsEnabled returns true if the library was build with the necessary
// flags to enable the FFI features necessary for parsing instructions.
func IsEnabled() bool {
	return true
}
