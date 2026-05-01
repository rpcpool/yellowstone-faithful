package solanatxmetaparsers

import (
	"encoding/json"
	"errors"
	"testing"

	transaction_status_meta_serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
)

func TestDecodeProtobufTransactionError_AcceptsRawTransactionError(t *testing.T) {
	buf, err := (&transaction_status_meta_serde_agave.TransactionError__AccountInUse{}).BincodeSerialize()
	if err != nil {
		t.Fatalf("serialize raw transaction error: %v", err)
	}

	got, err := decodeProtobufTransactionError(buf)
	if err != nil {
		t.Fatalf("decode raw transaction error: %v", err)
	}
	if _, ok := got.(*transaction_status_meta_serde_agave.TransactionError__AccountInUse); !ok {
		t.Fatalf("unexpected decoded type: %T", got)
	}
}

func TestDecodeProtobufTransactionError_AcceptsWrappedResultErr(t *testing.T) {
	buf, err := (&transaction_status_meta_serde_agave.Result__Err{
		Value: &transaction_status_meta_serde_agave.TransactionError__AccountInUse{},
	}).BincodeSerialize()
	if err != nil {
		t.Fatalf("serialize wrapped result error: %v", err)
	}

	got, err := decodeProtobufTransactionError(buf)
	if err != nil {
		t.Fatalf("decode wrapped result error: %v", err)
	}
	if _, ok := got.(*transaction_status_meta_serde_agave.TransactionError__AccountInUse); !ok {
		t.Fatalf("unexpected decoded type: %T", got)
	}
}

func TestDecodeProtobufTransactionError_ClassifiesIncompletePayloadAsUnsupported(t *testing.T) {
	_, err := decodeProtobufTransactionError([]byte{8, 0, 0, 0})
	if !errors.Is(err, ErrUnsupportedProtobufTransactionErrorPayload) {
		t.Fatalf("expected unsupported payload error, got: %v", err)
	}
}

func TestDecodeProtobufTransactionError_DecodesLegacyBorshIoError(t *testing.T) {
	// 9-byte pattern: InstructionError(2, BorshIoError) without string — old Solana format
	buf := []byte{8, 0, 0, 0, 2, 44, 0, 0, 0}
	got, err := decodeProtobufTransactionError(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	txInstrErr, ok := got.(*transaction_status_meta_serde_agave.TransactionError__InstructionError)
	if !ok {
		t.Fatalf("expected TransactionError__InstructionError, got %T", got)
	}
	if txInstrErr.ErrorCode != 2 {
		t.Fatalf("expected error code 2, got %d", txInstrErr.ErrorCode)
	}
	if _, ok := txInstrErr.Error.(*transaction_status_meta_serde_agave.InstructionError__BorshIoErrorLegacy); !ok {
		t.Fatalf("expected InstructionError__BorshIoErrorLegacy, got %T", txInstrErr.Error)
	}
}

func TestProtobufTransactionStatusMetaToUi_DecodesLegacyBorshIoError(t *testing.T) {
	raw, err := ProtobufTransactionStatusMetaToUi(&confirmed_block.TransactionStatusMeta{
		Err: &confirmed_block.TransactionError{
			Err: []byte{8, 0, 0, 0, 2, 44, 0, 0, 0},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	// Should produce {"InstructionError": [2, "BorshIoError"]} matching the canonical RPC form
	instrErr, ok := got["err"].(map[string]any)
	if !ok {
		t.Fatalf("expected err to be an object, got %T: %#v", got["err"], got["err"])
	}
	arr, ok := instrErr["InstructionError"].([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("expected InstructionError array, got %#v", instrErr)
	}
	if arr[0].(float64) != 2 {
		t.Fatalf("expected instruction index 2, got %v", arr[0])
	}
	if arr[1] != "BorshIoError" {
		t.Fatalf("expected \"BorshIoError\" string, got %#v", arr[1])
	}
}

func TestProtobufTransactionStatusMetaToUi_FallsBackForMalformedErrorPayload(t *testing.T) {
	raw, err := ProtobufTransactionStatusMetaToUi(&confirmed_block.TransactionStatusMeta{
		Err: &confirmed_block.TransactionError{
			Err: []byte{1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	if got["err"] != "UnsupportedTransactionError" {
		t.Fatalf("unexpected err field: %#v", got["err"])
	}

	status, ok := got["status"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected status field type: %T", got["status"])
	}
	if status["Err"] != "UnsupportedTransactionError" {
		t.Fatalf("unexpected status.Err field: %#v", status["Err"])
	}
}
