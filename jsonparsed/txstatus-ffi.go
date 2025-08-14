//go:build ffi
// +build ffi

package jsonparsed

/*
#cgo LDFLAGS: -L./lib -lsolana_transaction_status_wrapper -lm -ldl
#include "./lib/transaction_status.h"
*/
import "C"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
	"unsafe"

	"github.com/davecgh/go-spew/spew"
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

	startedParsingAt := time.Now()
	got := C.parse_instruction(cs, C.ulong(len(buf.Bytes())))
	if got.status == 0 {
		debugln("[golang] got status (OK):", got.status)
	} else {
		debugln("[golang] got status (ERR):", got.status)
	}
	debugln("[golang] got parsed instruction in:", time.Since(startedParsingAt))

	parsedInstructionJSON := C.GoBytes(unsafe.Pointer(got.buf.data), C.int(got.buf.len))
	debugln("[golang] got parsed instruction as json:", spew.Sdump(parsedInstructionJSON))
	debugln("[golang] got parsed instruction as json:", string(parsedInstructionJSON))

	return parsedInstructionJSON, nil
}

// IsEnabled returns true if the library was build with the necessary
// flags to enable the FFI features necessary for parsing instructions.
func IsEnabled() bool {
	return true
}
