use {
    old_faithful_proto::{decode, prost::Message, proto},
    serde::{
        de::{self, Deserializer},
        Deserialize,
    },
    solana_clock::{Slot, UnixTimestamp},
    solana_hash::Hash,
    solana_message::{
        v1::{Message as MessageV1, TransactionConfig},
        MessageHeader, VersionedMessage,
    },
    solana_storage_proto::convert::generated,
    solana_transaction::versioned::VersionedTransaction,
    solana_transaction_status::{RewardType, TransactionStatusMeta, TransactionWithStatusMeta},
};

#[derive(Debug, Deserialize)]
struct FixtureItem<T> {
    name: String,
    value: T,
}

#[derive(Debug, Deserialize)]
struct FixtureConfirmedBlock {
    #[serde(deserialize_with = "deserialize_hex")]
    previous_blockhash: Vec<u8>,
    #[serde(deserialize_with = "deserialize_hex")]
    blockhash: Vec<u8>,
    parent_slot: Slot,
    slot: Slot,
    block_time: UnixTimestamp,
    block_height: Slot,
    transactions: Vec<FixtureConfirmedTransactionInner>,
    #[serde(deserialize_with = "deserialize_hex")]
    rewards: Vec<u8>,
    num_partitions: Option<u64>,
}

impl From<FixtureConfirmedBlock> for proto::BlockResponse {
    fn from(data: FixtureConfirmedBlock) -> Self {
        proto::BlockResponse {
            previous_blockhash: data.previous_blockhash,
            blockhash: data.blockhash,
            parent_slot: data.parent_slot,
            slot: data.slot,
            block_time: data.block_time,
            block_height: data.block_height,
            transactions: data.transactions.into_iter().map(Into::into).collect(),
            rewards: data.rewards,
            num_partitions: data.num_partitions,
        }
    }
}

#[derive(Debug, Deserialize)]
struct FixtureConfirmedTransaction {
    transaction: FixtureConfirmedTransactionInner,
    slot: Slot,
    block_time: UnixTimestamp,
    index: Option<u64>,
}

impl From<FixtureConfirmedTransaction> for proto::TransactionResponse {
    fn from(data: FixtureConfirmedTransaction) -> Self {
        proto::TransactionResponse {
            transaction: Some(data.transaction.into()),
            slot: data.slot,
            block_time: data.block_time,
            index: data.index,
        }
    }
}

#[derive(Debug, Deserialize)]
struct FixtureConfirmedTransactionInner {
    #[serde(deserialize_with = "deserialize_hex")]
    transaction: Vec<u8>,
    #[serde(deserialize_with = "deserialize_hex")]
    meta: Vec<u8>,
    index: Option<u64>,
}

impl From<FixtureConfirmedTransactionInner> for proto::Transaction {
    fn from(data: FixtureConfirmedTransactionInner) -> Self {
        proto::Transaction {
            transaction: data.transaction,
            meta: data.meta,
            index: data.index,
        }
    }
}

fn deserialize_hex<'de, D>(deserializer: D) -> Result<Vec<u8>, D::Error>
where
    D: Deserializer<'de>,
{
    let input = String::deserialize(deserializer)?;
    const_hex::decode(input)
        .map_err(|error| de::Error::custom(format!("failed to decode hex: {error:?}")))
}

#[test]
fn confirmed_block() {
    let items: Vec<FixtureItem<FixtureConfirmedBlock>> =
        serde_json::from_str(include_str!("decode_confirmed_block.json"))
            .expect("invalid confirmed blocks");

    for item in items {
        let response: proto::BlockResponse = item.value.into();
        let result = decode::confirmed_block(&response);
        assert!(result.is_ok(), "failed to decode {}", item.name);
    }
}

#[test]
fn confirmed_transaction() {
    let items: Vec<FixtureItem<FixtureConfirmedTransaction>> =
        serde_json::from_str(include_str!("decode_confirmed_transaction.json"))
            .expect("invalid confirmed transactions");

    for item in items {
        let response: proto::TransactionResponse = item.value.into();
        let result = decode::confirmed_transaction(&response);
        assert!(result.is_ok(), "failed to decode {}", item.name);
    }
}

#[test]
fn confirmed_block_preserves_deactivated_stake_reward() {
    let rewards = generated::Rewards {
        rewards: vec![generated::Reward {
            pubkey: "stake-account".to_owned(),
            lamports: -42,
            post_balance: 1_000,
            reward_type: generated::RewardType::DeactivatedStake as i32,
            commission: "7".to_owned(),
            commission_bps: "725".to_owned(),
        }],
        num_partitions: Some(generated::NumPartitions { num_partitions: 1 }),
    }
    .encode_to_vec();
    let response = proto::BlockResponse {
        previous_blockhash: vec![1; 32],
        blockhash: vec![2; 32],
        parent_slot: 1,
        slot: 2,
        block_time: 3,
        block_height: 4,
        transactions: vec![],
        rewards,
        num_partitions: Some(1),
    };

    let block = decode::confirmed_block(&response).expect("block should decode");
    let reward = &block.rewards[0];
    assert_eq!(reward.reward_type, Some(RewardType::DeactivatedStake));
    assert_eq!(reward.commission, Some(7));
    assert_eq!(reward.commission_bps, Some(725));
    assert_eq!(reward.lamports, -42);
}

#[test]
fn confirmed_transaction_decodes_wincode_v1_and_preserves_index() {
    let transaction = VersionedTransaction {
        signatures: vec![],
        message: VersionedMessage::V1(MessageV1 {
            header: MessageHeader::default(),
            config: TransactionConfig {
                priority_fee: Some(9),
                compute_unit_limit: Some(200_000),
                loaded_accounts_data_size_limit: Some(32_768),
                heap_size: Some(64 * 1024),
            },
            lifetime_specifier: Hash::new_from_array([3; 32]),
            account_keys: vec![],
            instructions: vec![],
        }),
    };
    let meta: generated::TransactionStatusMeta = TransactionStatusMeta::default().into();
    let response = proto::TransactionResponse {
        transaction: Some(proto::Transaction {
            transaction: wincode::serialize(&transaction)
                .expect("V1 transaction should wincode encode"),
            meta: meta.encode_to_vec(),
            index: Some(4),
        }),
        slot: 5,
        block_time: 6,
        index: Some(7),
    };

    let decoded = decode::confirmed_transaction(&response).expect("V1 transaction should decode");
    assert_eq!(decoded.index, 7);
    let TransactionWithStatusMeta::Complete(transaction) = decoded.tx_with_meta else {
        panic!("metadata should be complete");
    };
    let VersionedMessage::V1(message) = transaction.transaction.message else {
        panic!("transaction should remain V1");
    };
    assert_eq!(message.config.priority_fee, Some(9));
    assert_eq!(message.config.compute_unit_limit, Some(200_000));
    assert_eq!(message.config.loaded_accounts_data_size_limit, Some(32_768));
    assert_eq!(message.config.heap_size, Some(64 * 1024));
}
