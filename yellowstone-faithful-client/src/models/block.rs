use {
    super::transaction::TransactionInfo,
    serde::{Deserialize, Serialize},
    solana_sdk::hash::Hash,
};

/// Block information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Block {
    /// Previous block hash, or None if there is no previous block (e.g., genesis block)
    pub previous_blockhash: Option<Hash>,

    /// Current block hash
    pub blockhash: Hash,

    /// Parent slot number
    pub parent_slot: u64,

    /// Current slot number
    pub slot: u64,

    /// Block timestamp (Unix timestamp)
    pub block_time: Option<i64>,

    /// Block height
    pub block_height: Option<u64>,

    /// Transactions in the block
    pub transactions: Vec<TransactionInfo>,

    /// Rewards data (serialized)
    pub rewards: Option<Vec<u8>>,

    /// Number of partitions (for sharded blocks)
    pub num_partitions: Option<u64>,
}

impl Block {
    /// Get the number of transactions in the block
    pub fn transaction_count(&self) -> usize {
        self.transactions.len()
    }

    /// Check if the block is empty
    pub fn is_empty(&self) -> bool {
        self.transactions.is_empty()
    }
}

impl Block {
    /// Convert from gRPC BlockResponse
    /// Handles empty previous_blockhash as None; errors on other invalid lengths
    pub fn from_grpc(
        response: crate::grpc::generated::BlockResponse,
    ) -> crate::error::Result<Self> {
        use crate::error::FaithfulError;

        tracing::debug!(
            "Parsing block response: slot={}, blockhash_len={}, prev_blockhash_len={}, tx_count={}",
            response.slot,
            response.blockhash.len(),
            response.previous_blockhash.len(),
            response.transactions.len()
        );

        // Parse previous_blockhash: allow empty (None) or exactly 32 bytes (Some(Hash))
        let previous_blockhash = match response.previous_blockhash.len() {
            0 => {
                tracing::warn!(
                    "Empty previous_blockhash for slot {} - treating as None (likely genesis or special block)",
                    response.slot
                );
                None
            }
            32 => {
                let hash_array: [u8; 32] = response
                    .previous_blockhash
                    .as_slice()
                    .try_into()
                    .map_err(|_| {
                        tracing::error!(
                            "Failed to convert previous_blockhash to [u8; 32] for slot {}",
                            response.slot
                        );
                        FaithfulError::InvalidResponse(format!(
                            "Failed to convert previous_blockhash to fixed array for slot {}",
                            response.slot
                        ))
                    })?;
                Some(Hash::from(hash_array))
            }
            len => {
                tracing::error!(
                    "Invalid previous_blockhash length for slot {}: expected 0 or 32 bytes, got {}",
                    response.slot,
                    len
                );
                return Err(FaithfulError::InvalidResponse(format!(
                    "Invalid previous_blockhash length for slot {}: expected 0 or 32 bytes, got {}",
                    response.slot, len
                )));
            }
        };

        // Validate and parse blockhash
        if response.blockhash.len() != 32 {
            tracing::error!(
                "Invalid blockhash length for slot {}: expected 32 bytes, got {}",
                response.slot,
                response.blockhash.len()
            );
            return Err(FaithfulError::InvalidResponse(format!(
                "Invalid blockhash length for slot {}: expected 32 bytes, got {}",
                response.slot,
                response.blockhash.len()
            )));
        }

        let blockhash_array: [u8; 32] = response.blockhash.as_slice().try_into().map_err(|_| {
            tracing::error!(
                "Failed to convert blockhash to [u8; 32] for slot {}",
                response.slot
            );
            FaithfulError::InvalidResponse(format!(
                "Failed to convert blockhash to fixed array for slot {}",
                response.slot
            ))
        })?;

        let blockhash = Hash::from(blockhash_array);

        tracing::debug!(
            "Successfully parsed hashes for slot {}: previous_blockhash={:?}, blockhash={}",
            response.slot,
            previous_blockhash,
            blockhash
        );

        // Parse transactions with error handling
        let transactions = response
            .transactions
            .into_iter()
            .enumerate()
            .map(|(idx, tx)| {
                TransactionInfo::from_grpc(tx).map_err(|e| {
                    tracing::error!(
                        "Failed to parse transaction {} in slot {}: {}",
                        idx,
                        response.slot,
                        e
                    );
                    e
                })
            })
            .collect::<crate::error::Result<Vec<_>>>()?;

        tracing::debug!(
            "Successfully parsed block for slot {} with {} transactions",
            response.slot,
            transactions.len()
        );

        Ok(Self {
            previous_blockhash,
            blockhash,
            parent_slot: response.parent_slot,
            slot: response.slot,
            block_time: if response.block_time == 0 {
                None
            } else {
                Some(response.block_time)
            },
            block_height: if response.block_height == 0 {
                None
            } else {
                Some(response.block_height)
            },
            transactions,
            rewards: if response.rewards.is_empty() {
                None
            } else {
                Some(response.rewards)
            },
            num_partitions: response.num_partitions,
        })
    }
}
