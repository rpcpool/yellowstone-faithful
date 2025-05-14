package solanatxmetaparsers

import (
	"encoding/json"
	"fmt"

	transaction_status_meta_serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
)

func ErrorToUi(
	txError transaction_status_meta_serde_agave.TransactionError,
) (json.RawMessage, error) {
	raw, err := txError.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction error: %w", err)
	}
	return raw, nil
}
