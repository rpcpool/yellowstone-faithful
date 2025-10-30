use {
    serde::{Deserialize, Serialize},
    solana_sdk::signature::Signature,
};

/// Transaction information
#[non_exhaustive]
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionInfo {
    /// Serialized transaction data
    pub transaction: Vec<u8>,

    /// Transaction metadata (serialized)
    pub meta: Option<Vec<u8>>,

    /// Position in the block
    pub index: Option<u64>,
}

/// Transaction with context (slot, block time)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionWithContext {
    /// The transaction itself
    pub transaction: TransactionInfo,

    /// Slot number where the transaction was included
    pub slot: u64,

    /// Block timestamp
    pub block_time: Option<i64>,

    /// Position in the block
    pub index: Option<u64>,
}

impl TransactionInfo {
    /// Create a new TransactionInfo
    pub fn new(transaction: Vec<u8>, meta: Option<Vec<u8>>, index: Option<u64>) -> Self {
        Self {
            transaction,
            meta,
            index,
        }
    }

    /// Get the transaction signature if available
    pub fn signature(&self) -> crate::error::Result<Signature> {
        use crate::error::FaithfulError;

        // Try to decode the transaction to extract the signature
        if self.transaction.len() < 64 {
            return Err(FaithfulError::InvalidResponse(
                "Transaction data too short".to_string(),
            ));
        }

        // The first 64 bytes are typically the signature
        let sig_bytes: [u8; 64] = self.transaction[..64]
            .try_into()
            .map_err(|_| FaithfulError::InvalidResponse("Invalid signature".to_string()))?;

        Ok(Signature::from(sig_bytes))
    }
}

impl TransactionInfo {
    /// Convert from gRPC Transaction
    pub fn from_grpc(tx: crate::grpc::generated::Transaction) -> crate::error::Result<Self> {
        Ok(Self {
            transaction: tx.transaction,
            meta: if tx.meta.is_empty() {
                None
            } else {
                Some(tx.meta)
            },
            index: tx.index,
        })
    }
}

impl TransactionWithContext {
    /// Convert from gRPC TransactionResponse
    pub fn from_grpc(
        response: crate::grpc::generated::TransactionResponse,
    ) -> crate::error::Result<Self> {
        let transaction = response.transaction.ok_or_else(|| {
            crate::error::FaithfulError::InvalidResponse(
                "Missing transaction in response".to_string(),
            )
        })?;

        Ok(Self {
            transaction: TransactionInfo::from_grpc(transaction)?,
            slot: response.slot,
            block_time: if response.block_time == 0 {
                None
            } else {
                Some(response.block_time)
            },
            index: response.index,
        })
    }
}
