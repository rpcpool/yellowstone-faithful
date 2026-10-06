package solanatxmetaparsers

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rpcpool/yellowstone-faithful/jsonparsed"
	"github.com/stretchr/testify/require"
)

// wireTx builds a signed transfer, serializes it and decodes it back the same
// way the CAR reader does, so the message version comes from the wire bytes.
func wireTx(t *testing.T, opt solana.TransactionOption) (*solana.Transaction, []byte) {
	t.Helper()
	payerKey := solana.PrivateKey(ed25519.NewKeyFromSeed(make([]byte, 32)))
	payer := payerKey.PublicKey()
	recipient := solana.MustPublicKeyFromBase58("2mHtsPqiHkQKKh6t2Q1jGwYQ8vG7ULfF7c9k4t9BvGkw")
	ix := system.NewTransferInstruction(1000, payer, recipient).Build()

	tx, err := solana.NewTransaction([]solana.Instruction{ix}, solana.Hash{1, 2, 3}, solana.TransactionPayer(payer), opt)
	require.NoError(t, err)
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer) {
			return &payerKey
		}
		return nil
	})
	require.NoError(t, err)

	wire, err := tx.MarshalBinary()
	require.NoError(t, err)
	decoded := &solana.Transaction{}
	require.NoError(t, bin.UnmarshalBin(decoded, wire))
	return decoded, wire
}

func v1Tx(t *testing.T) (*solana.Transaction, []byte) {
	t.Helper()
	cfg := solana.TransactionConfig{}.WithComputeUnitLimit(600).WithLoadedAccountsDataSizeLimit(30720)
	tx, wire := wireTx(t, solana.TransactionV1Config(cfg))
	require.Equal(t, byte(0x81), wire[0])
	return tx, wire
}

func renderTx(t *testing.T, tx *solana.Transaction, encoding solana.EncodingType) map[string]any {
	t.Helper()
	obj, err := NewEncodedTransactionWithStatusMeta(tx, nil).ToUi(encoding, rpc.TransactionDetailsFull)
	require.NoError(t, err)
	raw, err := obj.MarshalJSON()
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestToUi_JSON_V1MatchesAgave(t *testing.T) {
	tx, _ := v1Tx(t)
	out := renderTx(t, tx, solana.EncodingJSON)

	require.Equal(t, float64(1), out["version"])
	msg := out["transaction"].(map[string]any)["message"].(map[string]any)
	require.NotContains(t, msg, "addressTableLookups")
	require.Equal(t, map[string]any{
		"priorityFee":                 nil,
		"computeUnitLimit":            float64(600),
		"loadedAccountsDataSizeLimit": float64(30720),
		"heapSize":                    nil,
	}, msg["transactionConfig"])

	instructions := msg["instructions"].([]any)
	require.Len(t, instructions, 1)
	require.Equal(t, float64(1), instructions[0].(map[string]any)["stackHeight"])
}

func TestToUi_JSON_V0AndLegacyUnchanged(t *testing.T) {
	cases := []struct {
		name        string
		version     solana.MessageVersion
		wantVersion any
		wantLookups bool
	}{
		{"legacy", solana.MessageVersionLegacy, "legacy", false},
		{"v0", solana.MessageVersionV0, float64(0), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, _ := wireTx(t, solana.TransactionMessageVersion(tc.version))
			out := renderTx(t, tx, solana.EncodingJSON)

			require.Equal(t, tc.wantVersion, out["version"])
			msg := out["transaction"].(map[string]any)["message"].(map[string]any)
			require.NotContains(t, msg, "transactionConfig")
			if tc.wantLookups {
				require.Equal(t, []any{}, msg["addressTableLookups"])
			} else {
				require.NotContains(t, msg, "addressTableLookups")
			}
			ix := msg["instructions"].([]any)[0].(map[string]any)
			require.Contains(t, ix, "stackHeight")
			require.Nil(t, ix["stackHeight"])
		})
	}
}

func TestToUi_Base64_V1(t *testing.T) {
	tx, wire := v1Tx(t)
	out := renderTx(t, tx, solana.EncodingBase64)

	require.Equal(t, float64(1), out["version"])
	encoded := out["transaction"].([]any)
	require.Equal(t, "base64", encoded[1])
	got, err := base64.StdEncoding.DecodeString(encoded[0].(string))
	require.NoError(t, err)
	require.Equal(t, wire, got)
}

func TestJsonParsed_V1MessageShape(t *testing.T) {
	tx, _ := v1Tx(t)

	parsedTx, err := jsonparsed.FromTransaction(tx)
	require.NoError(t, err)
	parsedTx.Message.Instructions = nil // ToUi replaces these with parsed instructions
	raw, err := json.Marshal(parsedTx)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	msg := out["message"].(map[string]any)
	require.NotContains(t, msg, "addressTableLookups")
	require.Equal(t, map[string]any{
		"priorityFee":                 nil,
		"computeUnitLimit":            float64(600),
		"loadedAccountsDataSizeLimit": float64(30720),
		"heapSize":                    nil,
	}, msg["transactionConfig"])

	ixRaw, err := compiledInstructionsToJsonParsed(tx, tx.Message.Instructions[0], nil, topLevelStackHeight(tx))
	require.NoError(t, err)
	var ix map[string]any
	require.NoError(t, json.Unmarshal(ixRaw, &ix))
	require.Equal(t, float64(1), ix["stackHeight"])

	legacy, _ := wireTx(t, solana.TransactionMessageVersion(solana.MessageVersionLegacy))
	parsedLegacy, err := jsonparsed.FromTransaction(legacy)
	require.NoError(t, err)
	parsedLegacy.Message.Instructions = nil
	raw, err = json.Marshal(parsedLegacy)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "transactionConfig")
}
