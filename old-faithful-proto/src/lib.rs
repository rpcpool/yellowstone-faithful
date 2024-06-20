pub use {prost, tonic};

pub mod proto {
    tonic::include_proto!("old_faithful");
}

pub mod decode {
    use {
        super::{
            proto,
            solana::{StoredConfirmedBlockRewards, StoredConfirmedBlockTransactionStatusMeta},
        },
        anyhow::Context,
        prost_011::Message,
        solana_sdk::{
            hash::{Hash, HASH_BYTES},
            transaction::VersionedTransaction,
        },
        solana_storage_proto::convert::generated,
        solana_transaction_status::{
            ConfirmedBlock, ConfirmedTransactionWithStatusMeta, Reward, TransactionStatusMeta,
            TransactionWithStatusMeta, VersionedTransactionWithStatusMeta,
        },
    };

    pub fn confirmed_block(block: proto::BlockResponse) -> anyhow::Result<ConfirmedBlock> {
        let previous_blockhash = <[u8; HASH_BYTES]>::try_from(block.previous_blockhash)
            .map_err(|_error| anyhow::anyhow!("failed to decode previous_blockhash"))?;
        let blockhash = <[u8; HASH_BYTES]>::try_from(block.blockhash)
            .map_err(|_error| anyhow::anyhow!("failed to decode blockhash"))?;

        let rewards: Vec<Reward> = match generated::Rewards::decode(block.rewards.as_ref()) {
            Ok(rewards) => rewards.into(),
            Err(_error) => bincode::deserialize::<StoredConfirmedBlockRewards>(&block.rewards)
                .context("failed to decode bincode of Vec<Reward>")?
                .into_iter()
                .map(Into::into)
                .collect(),
        };

        Ok(ConfirmedBlock {
            previous_blockhash: Hash::new_from_array(previous_blockhash).to_string(),
            blockhash: Hash::new_from_array(blockhash).to_string(),
            parent_slot: block.parent_slot,
            transactions: block
                .transactions
                .iter()
                .map(versioned_transaction)
                .collect::<Result<Vec<_>, _>>()?,
            rewards,
            block_time: Some(block.block_time),
            block_height: Some(block.block_height),
        })
    }

    pub fn confirmed_transaction(
        tx: &proto::TransactionResponse,
    ) -> anyhow::Result<ConfirmedTransactionWithStatusMeta> {
        versioned_transaction(
            tx.transaction
                .as_ref()
                .ok_or_else(|| anyhow::anyhow!("failed to get Transaction"))?,
        )
        .map(|tx_with_meta| ConfirmedTransactionWithStatusMeta {
            slot: tx.slot,
            tx_with_meta,
            block_time: Some(tx.block_time),
        })
    }

    pub fn versioned_transaction(
        tx: &proto::Transaction,
    ) -> anyhow::Result<TransactionWithStatusMeta> {
        let transaction: VersionedTransaction = bincode::deserialize(&tx.transaction)
            .context("failed to decode VersionedTransaction")?;

        let meta: TransactionStatusMeta =
            match generated::TransactionStatusMeta::decode(tx.meta.as_ref()) {
                Ok(meta) => meta
                    .try_into()
                    .context("failed to decode protobuf of TransactionStatusMeta")?,
                Err(_error) => {
                    bincode::deserialize::<StoredConfirmedBlockTransactionStatusMeta>(&tx.meta)
                        .context("failed to decode bincode of TransactionStatusMeta")?
                        .into()
                }
            };

        Ok(TransactionWithStatusMeta::Complete(
            VersionedTransactionWithStatusMeta { transaction, meta },
        ))
    }
}

mod solana {
    use {
        serde::{Deserialize, Serialize},
        solana_sdk::{message::v0::LoadedAddresses, transaction::TransactionError},
        solana_transaction_status::{Reward, TransactionStatusMeta},
    };

    #[derive(Serialize, Deserialize)]
    pub struct StoredConfirmedBlockTransactionStatusMeta {
        err: Option<TransactionError>,
        fee: u64,
        pre_balances: Vec<u64>,
        post_balances: Vec<u64>,
    }

    impl From<StoredConfirmedBlockTransactionStatusMeta> for TransactionStatusMeta {
        fn from(value: StoredConfirmedBlockTransactionStatusMeta) -> Self {
            let StoredConfirmedBlockTransactionStatusMeta {
                err,
                fee,
                pre_balances,
                post_balances,
            } = value;
            let status = match &err {
                None => Ok(()),
                Some(err) => Err(err.clone()),
            };
            Self {
                status,
                fee,
                pre_balances,
                post_balances,
                inner_instructions: None,
                log_messages: None,
                pre_token_balances: None,
                post_token_balances: None,
                rewards: None,
                loaded_addresses: LoadedAddresses::default(),
                return_data: None,
                compute_units_consumed: None,
            }
        }
    }

    pub type StoredConfirmedBlockRewards = Vec<StoredConfirmedBlockReward>;

    #[derive(Serialize, Deserialize)]
    pub struct StoredConfirmedBlockReward {
        pubkey: String,
        lamports: i64,
    }

    impl From<StoredConfirmedBlockReward> for Reward {
        fn from(value: StoredConfirmedBlockReward) -> Self {
            let StoredConfirmedBlockReward { pubkey, lamports } = value;
            Self {
                pubkey,
                lamports,
                post_balance: 0,
                reward_type: None,
                commission: None,
            }
        }
    }
}
