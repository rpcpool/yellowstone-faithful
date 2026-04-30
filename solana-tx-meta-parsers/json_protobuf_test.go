package solanatxmetaparsers

import (
	"encoding/json"
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
