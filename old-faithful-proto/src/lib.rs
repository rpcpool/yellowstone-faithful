pub use {prost, tonic};

pub mod proto {
    tonic::include_proto!("old_faithful");
}

pub mod decode {
    use {
        super::{proto, solana::StoredConfirmedBlockTransaction},
        anyhow::Context,
        prost_011::Message,
        solana_storage_proto::convert::generated,
        solana_transaction_status::{
            ConfirmedBlock, ConfirmedTransactionWithStatusMeta, TransactionWithStatusMeta,
        },
    };

    pub fn confirmed_block(block: proto::BlockResponse) -> anyhow::Result<ConfirmedBlock> {
        todo!()
    }

    pub fn confirmed_transaction(
        tx: proto::TransactionResponse,
    ) -> anyhow::Result<ConfirmedTransactionWithStatusMeta> {
        versioned_transaction(
            tx.transaction
                .ok_or_else(|| anyhow::anyhow!("failed to get Transaction"))?,
        )
        .map(|tx_with_meta| ConfirmedTransactionWithStatusMeta {
            slot: tx.slot,
            tx_with_meta,
            block_time: Some(tx.block_time),
        })
    }

    pub fn versioned_transaction(
        tx: proto::Transaction,
    ) -> anyhow::Result<TransactionWithStatusMeta> {
        Ok(
            match generated::ConfirmedTransaction::decode(tx.meta.as_ref()) {
                Ok(meta) => meta
                    .try_into()
                    .context("failed to decode protobuf struct to solana")?,
                Err(_error) => bincode::deserialize::<StoredConfirmedBlockTransaction>(&tx.meta)
                    .context("failed to decode with bincode")?
                    .into(),
            },
        )
    }
}

mod solana {
    use {
        serde::{Deserialize, Serialize},
        solana_sdk::{
            message::v0::LoadedAddresses,
            transaction::{TransactionError, VersionedTransaction},
        },
        solana_transaction_status::{
            TransactionStatusMeta, TransactionWithStatusMeta, VersionedTransactionWithStatusMeta,
        },
    };

    #[derive(Serialize, Deserialize)]
    pub struct StoredConfirmedBlockTransaction {
        transaction: VersionedTransaction,
        meta: Option<StoredConfirmedBlockTransactionStatusMeta>,
    }

    impl From<StoredConfirmedBlockTransaction> for TransactionWithStatusMeta {
        fn from(tx_with_meta: StoredConfirmedBlockTransaction) -> Self {
            let StoredConfirmedBlockTransaction { transaction, meta } = tx_with_meta;
            match meta {
                None => Self::MissingMetadata(
                    transaction
                        .into_legacy_transaction()
                        .expect("versioned transactions always have meta"),
                ),
                Some(meta) => Self::Complete(VersionedTransactionWithStatusMeta {
                    transaction,
                    meta: meta.into(),
                }),
            }
        }
    }

    #[derive(Serialize, Deserialize)]
    struct StoredConfirmedBlockTransactionStatusMeta {
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
}
