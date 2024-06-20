pub use {prost, tonic};

pub mod proto {
    tonic::include_proto!("old_faithful");
}

pub mod decode {
    use {
        super::{proto, solana::StoredConfirmedBlockTransactionStatusMeta},
        anyhow::Context,
        prost_011::Message,
        solana_sdk::transaction::VersionedTransaction,
        solana_storage_proto::convert::generated,
        solana_transaction_status::{
            ConfirmedBlock, ConfirmedTransactionWithStatusMeta, TransactionStatusMeta,
            TransactionWithStatusMeta, VersionedTransactionWithStatusMeta,
        },
    };

    pub fn confirmed_block(block: proto::BlockResponse) -> anyhow::Result<ConfirmedBlock> {
        todo!()
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
        solana_transaction_status::TransactionStatusMeta,
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
}
