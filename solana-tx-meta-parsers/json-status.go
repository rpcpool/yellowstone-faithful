package solanatxmetaparsers

import (
	"encoding/json"
	"errors"
	"fmt"

	transaction_status_meta_serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
)

func ErrorToUi(
	status transaction_status_meta_serde_agave.Result,
) (json.RawMessage, error) {
	switch status := status.(type) {
	case *transaction_status_meta_serde_agave.Result__Err:
		storedErr := status
		if storedErr == nil {
			return nil, errors.New("error is nil")
		}
		txError := storedErr.Value

		raw, err := txError.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal transaction error: %w", err)
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("unknown result type: %T", status)
	}
}
