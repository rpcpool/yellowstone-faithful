package transaction_status_meta_serde_agave

import (
	"errors"
	"fmt"
	"io"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/serde"
)

// // A serialized `StoredConfirmedBlock` is stored in the `block` table
// //
// // StoredConfirmedBlock holds the same contents as ConfirmedBlock, but is slightly compressed and avoids
// // some serde JSON directives that cause issues with bincode
// //
// // Note: in order to continue to support old bincode-serialized bigtable entries, if new fields are
// // added to ConfirmedBlock, they must either be excluded or set to `default_on_eof` here
// //
// #[derive(Serialize, Deserialize)]
//
//	struct StoredConfirmedBlock {
//	    previous_blockhash: String,
//	    blockhash: String,
//	    parent_slot: Slot,
//	    transactions: Vec<StoredConfirmedBlockTransaction>,
//	    rewards: StoredConfirmedBlockRewards,
//	    block_time: Option<UnixTimestamp>,
//	    #[serde(deserialize_with = "default_on_eof")]
//	    block_height: Option<u64>,
//	}
type StoredConfirmedBlock struct {
	PreviousBlockhash string
	Blockhash         string
	ParentSlot        uint64
	Transactions      []StoredConfirmedBlockTransaction
	Rewards           StoredConfirmedBlockRewards
	BlockTime         *int64
	BlockHeight       *uint64 // This field is optional and may not be present in all serialized
}

func BincodeDeserializeStoredConfirmedBlock(input []byte) (*StoredConfirmedBlock, error) {
	if input == nil {
		return nil, fmt.Errorf("cannot deserialize null array")
	}
	deserializer := NewDeserializer(input)
	obj, err := DeserializeStoredConfirmedBlock(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return nil, fmt.Errorf("some input bytes were not read")
	}
	return &obj, err
}

func DeserializeStoredConfirmedBlock(deserializer serde.Deserializer) (StoredConfirmedBlock, error) {
	var obj StoredConfirmedBlock
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, fmt.Errorf("failed to increase container depth at StoredConfirmedBlock: %w", err)
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.PreviousBlockhash = val
	} else {
		return obj, fmt.Errorf("failed to deserialize PreviousBlockhash: %w", err)
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.Blockhash = val
	} else {
		return obj, fmt.Errorf("failed to deserialize Blockhash: %w", err)
	}
	if val, err := deserializer.DeserializeU64(); err == nil {
		obj.ParentSlot = val
	} else {
		return obj, fmt.Errorf("failed to deserialize ParentSlot: %w", err)
	}
	if val, err := deserialize_vector_StoredConfirmedBlockTransaction(deserializer); err == nil {
		obj.Transactions = val
	} else {
		return obj, fmt.Errorf("failed to deserialize Transactions: %w", err)
	}
	if val, err := DeserializeStoredConfirmedBlockRewards(deserializer); err == nil {
		obj.Rewards = *val
	} else {
		return obj, fmt.Errorf("failed to deserialize Rewards: %w", err)
	}
	if val, err := deserialize_option_i64(deserializer); err == nil {
		obj.BlockTime = val
	} else {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "invalid bool byte") || strings.Contains(err.Error(), "length is too large") {
			obj.BlockTime = nil
			err = nil
		} else {
			return obj, fmt.Errorf("failed to deserialize BlockTime: %w", err)
		}
	}
	if val, err := deserialize_option_u64(deserializer); err == nil {
		obj.BlockHeight = val
	} else {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "invalid bool byte") || strings.Contains(err.Error(), "length is too large") {
			obj.BlockHeight = nil
			err = nil
		} else {
			return obj, fmt.Errorf("failed to deserialize BlockHeight: %w", err)
		}
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func deserialize_vector_StoredConfirmedBlockTransaction(deserializer serde.Deserializer) ([]StoredConfirmedBlockTransaction, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize length: %w", err)
	}
	obj := make([]StoredConfirmedBlockTransaction, length)
	for i := range obj {
		if val, err := DeserializeStoredConfirmedBlockTransaction(deserializer); err == nil {
			obj[i] = val
		} else {
			return nil, fmt.Errorf("failed to deserialize StoredConfirmedBlockTransaction[%d]: %w", i, err)
		}
	}
	return obj, nil
}

func DeserializeStoredConfirmedBlockTransaction(deserializer serde.Deserializer) (StoredConfirmedBlockTransaction, error) {
	var obj StoredConfirmedBlockTransaction
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, fmt.Errorf("failed to increase container depth at StoredConfirmedBlockTransaction: %w", err)
	}
	if val, err := DeserializeVersionedTransaction(deserializer); err == nil {
		obj.Transaction = val
	} else {
		return obj, fmt.Errorf("failed to deserialize Transaction: %w", err)
	}
	if val, err := deserialize_option_StoredConfirmedBlockTransactionStatusMeta(deserializer); err == nil {
		obj.Meta = val
	} else {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "invalid bool byte") || strings.Contains(err.Error(), "length is too large") {
			obj.Meta = nil
			err = nil
		} else {
			return obj, fmt.Errorf("failed to deserialize Meta: %w", err)
		}
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func deserialize_option_StoredConfirmedBlockTransactionStatusMeta(deserializer serde.Deserializer) (*StoredConfirmedBlockTransactionStatusMeta, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(StoredConfirmedBlockTransactionStatusMeta)
		if val, err := DeserializeStoredConfirmedBlockTransactionStatusMeta(deserializer); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

// #[derive(Serialize, Deserialize)]
//
//	struct StoredConfirmedBlockTransactionStatusMeta {
//	    err: Option<TransactionError>,
//	    fee: u64,
//	    pre_balances: Vec<u64>,
//	    post_balances: Vec<u64>,
//	}
//
// This is how tx meta is stored in bigtable (different from the one in RocksDB).
type StoredConfirmedBlockTransactionStatusMeta struct {
	Err          *TransactionError
	Fee          uint64
	PreBalances  []uint64
	PostBalances []uint64
}

func BincodeDeserializeStoredConfirmedBlockTransactionStatusMeta(input []byte) (StoredConfirmedBlockTransactionStatusMeta, error) {
	if input == nil {
		return StoredConfirmedBlockTransactionStatusMeta{}, fmt.Errorf("cannot deserialize null array")
	}
	deserializer := NewDeserializer(input)
	obj, err := DeserializeStoredConfirmedBlockTransactionStatusMeta(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return StoredConfirmedBlockTransactionStatusMeta{}, fmt.Errorf("some input bytes were not read")
	}
	return obj, err
}

func DeserializeStoredConfirmedBlockTransactionStatusMeta(de serde.Deserializer) (StoredConfirmedBlockTransactionStatusMeta, error) {
	var obj StoredConfirmedBlockTransactionStatusMeta
	if err := de.IncreaseContainerDepth(); err != nil {
		return obj, fmt.Errorf("failed to increase container depth at StoredConfirmedBlockTransactionStatusMeta: %w", err)
	}
	if val, err := deserialize_option_TransactionError(de); err == nil {
		obj.Err = val
	} else {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "invalid bool byte") || strings.Contains(err.Error(), "length is too large") {
			obj.Err = nil
			err = nil
		} else {
			return obj, fmt.Errorf("failed to deserialize Err: %w", err)
		}
	}
	if val, err := de.DeserializeU64(); err == nil {
		obj.Fee = val
	} else {
		return obj, fmt.Errorf("failed to deserialize Fee: %w", err)
	}
	if val, err := deserialize_vector_u64(de); err == nil {
		obj.PreBalances = val
	} else {
		return obj, fmt.Errorf("failed to deserialize PreBalances: %w", err)
	}
	if val, err := deserialize_vector_u64(de); err == nil {
		obj.PostBalances = val
	} else {
		return obj, fmt.Errorf("failed to deserialize PostBalances: %w", err)
	}
	de.DecreaseContainerDepth()

	return obj, nil
}

func deserialize_option_TransactionError(deserializer serde.Deserializer) (*TransactionError, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(TransactionError)
		if val, err := DeserializeTransactionError(deserializer); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func deserialize_option_i64(deserializer serde.Deserializer) (*int64, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(int64)
		if val, err := deserializer.DeserializeI64(); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

// #[derive(Serialize, Deserialize)]
// struct StoredConfirmedBlockTransaction {
//     transaction: VersionedTransaction,
//     meta: Option<StoredConfirmedBlockTransactionStatusMeta>,
// }

type StoredConfirmedBlockTransaction struct {
	Transaction *solana.Transaction
	Meta        *StoredConfirmedBlockTransactionStatusMeta
}

// /// Bit mask that indicates whether a serialized message is versioned.
// pub const MESSAGE_VERSION_PREFIX: u8 = 0x80;

// /// Either a legacy message or a v0 message.
// ///
// /// # Serialization
// ///
// /// If the first bit is set, the remaining 7 bits will be used to determine
// /// which message version is serialized starting from version `0`. If the first
// /// is bit is not set, all bytes are used to encode the legacy `Message`
// /// format.
// #[cfg_attr(
//     feature = "frozen-abi",
//     frozen_abi(digest = "2RTtea34NPrb8p9mWHCWjFh76cwP3MbjSmeoj5CXEBwN"),
//     derive(AbiEnumVisitor, AbiExample)
// )]
// #[derive(Debug, PartialEq, Eq, Clone)]
// pub enum VersionedMessage {
//     Legacy(LegacyMessage),
//     V0(v0::Message),
// }

func parseTransactionFromDeserializer(des serde.Deserializer) (*solana.Transaction, error) {
	desReal := des.(*deserializer)

	// 1. View all unread bytes from the deserializer WITHOUT consuming them.
	unreadData := desReal.Buffer.Bytes()

	// 2. Create a new decoder that operates on this temporary slice view.
	bidec := bin.NewBinDecoder(unreadData)

	// 3. Deserialize the transaction.
	tx, err := solana.TransactionFromDecoder(bidec)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %w", err)
	}

	// 4. Calculate how many bytes were actually consumed by the decoder.
	bytesConsumed := len(unreadData) - bidec.Remaining()

	// 5. Advance the original buffer by the number of bytes consumed.
	// This effectively removes the consumed bytes from the buffer, leaving only
	// the remaining data for the next read.
	desReal.Buffer.Next(bytesConsumed)

	return tx, nil
}

func DeserializeVersionedTransaction(deserializer serde.Deserializer) (*solana.Transaction, error) {
	return parseTransactionFromDeserializer(deserializer)
}
