use {
    old_faithful_proto::{decode, proto},
    serde::{
        de::{self, Deserializer},
        Deserialize,
    },
    solana_sdk::clock::{Slot, UnixTimestamp},
};

#[derive(Debug, Deserialize)]
struct FixtureItem<T> {
    name: String,
    value: T,
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
