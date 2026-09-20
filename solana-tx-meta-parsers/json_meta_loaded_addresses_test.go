package solanatxmetaparsers

import (
	"encoding/json"
	"testing"

	"github.com/rpcpool/yellowstone-faithful/third_party/solana_proto/confirmed_block"
)

func TestProtobufTransactionStatusMetaToUi_OmitsLoadedAddressesForJsonParsed(t *testing.T) {
	writable := []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	readonly := []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}

	rawOmit, err := ProtobufTransactionStatusMetaToUi(&confirmed_block.TransactionStatusMeta{
		LoadedWritableAddresses: [][]byte{writable},
		LoadedReadonlyAddresses: [][]byte{readonly},
	}, true)
	if err != nil {
		t.Fatalf("omit loaded addresses: %v", err)
	}

	var omit map[string]any
	if err := json.Unmarshal(rawOmit, &omit); err != nil {
		t.Fatalf("unmarshal omit json: %v", err)
	}
	if _, ok := omit["loadedAddresses"]; ok {
		t.Fatalf("expected loadedAddresses omitted, got %#v", omit["loadedAddresses"])
	}

	rawInclude, err := ProtobufTransactionStatusMetaToUi(&confirmed_block.TransactionStatusMeta{
		LoadedWritableAddresses: [][]byte{writable},
		LoadedReadonlyAddresses: [][]byte{readonly},
	}, false)
	if err != nil {
		t.Fatalf("include loaded addresses: %v", err)
	}

	var include map[string]any
	if err := json.Unmarshal(rawInclude, &include); err != nil {
		t.Fatalf("unmarshal include json: %v", err)
	}
	if _, ok := include["loadedAddresses"]; !ok {
		t.Fatalf("expected loadedAddresses present when omitLoadedAddresses=false")
	}
}
