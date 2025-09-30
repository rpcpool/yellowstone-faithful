use {
    crossbeam_channel::unbounded,
    oldfaithful_geyser_runner::{node, utils},
    solana_rpc::optimistically_confirmed_bank_tracker::SlotNotification,
    solana_runtime::bank::KeyedRewardsAndNumPartitions,
    solana_sdk::{reward_info::RewardInfo, reward_type::RewardType, signature::Signature},
    std::{
        collections::HashSet,
        convert::{TryFrom, TryInto},
        env::args,
        error::Error,
        io::BufReader,
        str::FromStr,
    },
};

fn main() -> Result<(), Box<dyn Error>> {
    use tracing_subscriber::{fmt, EnvFilter};
    // Build a subscriber that prints to stderr and obeys RUST_LOG.
    fmt().with_env_filter(EnvFilter::from_default_env()).init();

    let file_path = args().nth(1).expect("no file or url given");
    let _started_at = std::time::Instant::now();
    let file = open_reader(&file_path)?;
    let reader = BufReader::with_capacity(8 * 1024 * 1024, file);
    let mut item_index = 0;
    {
        let mut reader = node::NodeReader::new(reader)?;
        let header = reader.read_raw_header()?;
        println!("Header bytes: {:?}", header);

        let geyser_config_files = &[std::path::PathBuf::from(args().nth(2).unwrap())];

        let (confirmed_bank_sender, confirmed_bank_receiver) = unbounded();
        // drop(confirmed_bank_sender);
        let service =
            solana_geyser_plugin_manager::geyser_plugin_service::GeyserPluginService::new(
                confirmed_bank_receiver,
                geyser_config_files,
            )
            .unwrap_or_else(|err| panic!("Failed to create GeyserPluginService, error: {:?}", err));

        let transaction_notifier = service
            .get_transaction_notifier()
            .ok_or_else(|| panic!("Failed to get transaction notifier from GeyserPluginService"))
            .unwrap();

        let entry_notifier_maybe = service.get_entry_notifier();
        if entry_notifier_maybe.is_some() {
            println!("Entry notifications enabled")
        } else {
            println!("None of the plugins have enabled entry notifications")
        }

        let block_meta_notifier_maybe = service.get_block_metadata_notifier();

        let mut todo_previous_blockhash = solana_sdk::hash::Hash::default();
        let mut todo_latest_entry_blockhash = solana_sdk::hash::Hash::default();
        loop {
            let nodes = reader.read_until_block().map_err(|err| {
                Box::new(std::io::Error::new(
                    std::io::ErrorKind::Other,
                    std::format!("Error reading until block: {:?}", err),
                ))
            })?;
            // println!("Nodes: {:?}", nodes.get_cids());
            let block = nodes.get_block().map_err(|err| {
                Box::new(std::io::Error::new(
                    std::io::ErrorKind::Other,
                    std::format!("Error reading block: {:?}", err),
                ))
            })?;

            println!("Slot: {:?}", block.slot);
            // println!("Raw node: {:?}", raw_node);
            let mut entry_index: usize = 0;
            let mut this_block_executed_transaction_count: u64 = 0;
            let mut this_block_entry_count: u64 = 0;
            let mut this_block_rewards: solana_storage_proto::convert::generated::Rewards =
                solana_storage_proto::convert::generated::Rewards::default();
            nodes.each(|node_with_cid| -> Result<(), Box<dyn Error>> {
                item_index += 1;
                let node = node_with_cid.get_node();

                match node {
                    node::Node::Transaction(transaction) => {
                        let parsed = transaction.as_parsed()?;

                        {
                            let reassembled_metadata =
                                nodes.reassemble_dataframes(transaction.metadata.clone()).map_err(|err| {
                                    Box::new(std::io::Error::new(
                                        std::io::ErrorKind::Other,
                                        std::format!("Error reassembling metadata: {:?}", err),
                                    ))
                                })?;

                            let decompressed = if !reassembled_metadata.is_empty() {
                                utils::decompress_zstd(reassembled_metadata.clone()).map_err(|err| {
                                    Box::new(std::io::Error::new(
                                            std::io::ErrorKind::Other,
                                            std::format!("Error decompressing metadata: {:?}", err),
                                    ))
                                })?
                            } else {
                                Default::default()
                            };

                            let metadata: solana_storage_proto::convert::generated::TransactionStatusMeta =
                                prost_011::Message::decode(decompressed.as_slice()).map_err(|err| {
                                    Box::new(std::io::Error::new(
                                        std::io::ErrorKind::Other,
                                        std::format!("Error decoding metadata: {:?}", err),
                                    ))
                                })?;


                            let as_native_metadata: solana_transaction_status::TransactionStatusMeta =
                                metadata.try_into().map_err(|err| {
                                    Box::new(std::io::Error::new(
                                        std::io::ErrorKind::Other,
                                        std::format!("Error converting metadata to native: {:?}", err),
                                    ))
                                })?;

                            let is_vote = is_simple_vote_transaction(&parsed);

                           {
                                // TODO: test address loading.
                                let dummy_address_loader = MessageAddressLoaderFromTxMeta::new(as_native_metadata.clone());
                                let sanitized_tx = match  parsed.version() {
                                    solana_sdk::transaction::TransactionVersion::Number(_)=> {
                                        let message_hash = parsed.verify_and_hash_message()?;
                                        let versioned_sanitized_tx= solana_sdk::transaction::SanitizedVersionedTransaction::try_from(parsed)?;
                                        solana_sdk::transaction::SanitizedTransaction::try_new(
                                            versioned_sanitized_tx,
                                            message_hash,
                                            false,
                                            dummy_address_loader,
                                            &HashSet::default(),
                                        )
                                    },
                                    solana_sdk::transaction::TransactionVersion::Legacy(_legacy)=> {
                                        let message_hash = parsed.verify_and_hash_message()?;
                                        let versioned_sanitized_tx= solana_sdk::transaction::SanitizedVersionedTransaction::try_from(parsed)?;
                                        solana_sdk::transaction::SanitizedTransaction::try_new(
                                            versioned_sanitized_tx,
                                            message_hash,
                                            is_vote,
                                            dummy_address_loader,
                                            &HashSet::default(),
                                        )
                                    },
                                };
                                if sanitized_tx.is_err() {
                                    panic!(
                                        "Failed to create SanitizedTransaction, error: {:?}",
                                        sanitized_tx.err()
                                    );
                                }
                                let sanitized_tx = sanitized_tx.unwrap();

                                transaction_notifier
                                        .notify_transaction(
                                            block.slot,
                                            transaction.index.unwrap() as usize,
                                            sanitized_tx.signature(),
                                            &as_native_metadata,
                                            &sanitized_tx,
                                        );
                            }
                        }

                        // if parsed.version()
                        //     == solana_sdk::transaction::TransactionVersion::Number(0)
                        // {
                        //     return Ok(());
                        // }
                    }
                    node::Node::Entry(_entry) => {
                        todo_latest_entry_blockhash = solana_sdk::hash::Hash::from(_entry.hash.to_bytes());
                        this_block_executed_transaction_count += _entry.transactions.len() as u64;
                        this_block_entry_count += 1;
                        if entry_notifier_maybe.is_none() {
                            return Ok(());
                        }
                        let entry_notifier = entry_notifier_maybe.as_ref().unwrap();
                        // println!("___ Entry: {:?}", entry);
                        let entry_summary=solana_entry::entry::EntrySummary {
                            num_hashes: _entry.num_hashes,
                            hash: solana_sdk::hash::Hash::from(_entry.hash.to_bytes()),
                            num_transactions: _entry.transactions.len() as u64,
                        };

                        let starting_transaction_index = 0; // TODO:: implement this
                        entry_notifier
                            .notify_entry(block.slot, entry_index  ,&entry_summary, starting_transaction_index);
                        entry_index+=1;
                    }
                    node::Node::Block(_block) => {
                        // println!("___ Block: {:?}", block);
                        let notification = SlotNotification::Root((block.slot, block.meta.parent_slot));
                        confirmed_bank_sender.send(notification).unwrap();

                        {
                            if block_meta_notifier_maybe.is_none() {
                                return Ok(());
                            }
                            let mut keyed_rewards = Vec::with_capacity(this_block_rewards.rewards.len());
                            {
                                // convert this_block_rewards to rewards
                                for this_block_reward in this_block_rewards.rewards.iter() {
                                    let reward: RewardInfo = RewardInfo{
                                        reward_type: match this_block_reward.reward_type {
                                            0 => RewardType::Fee,
                                            1 => RewardType::Rent,
                                            2 => RewardType::Staking,
                                            3 => RewardType::Voting,
                                            _ => panic!("___ not supported reward type"),
                                        },
                                        lamports: this_block_reward.lamports,
                                        post_balance: this_block_reward.post_balance,
                                        // commission is Option<u8> , but this_block_reward.commission is string
                                        commission: match this_block_reward.commission.parse::<u8>() {
                                            Ok(commission) => Some(commission),
                                            Err(_err) => None,
                                        },
                                    };
                                    keyed_rewards.push((solana_sdk::pubkey::Pubkey::from_str(&this_block_reward.pubkey)?, reward));
                                }
                            }
                            // if keyed_rewards.read().unwrap().len() > 0 {
                            //   panic!("___ Rewards: {:?}", keyed_rewards.read().unwrap());
                            // }
                            let block_meta_notifier = block_meta_notifier_maybe.as_ref().unwrap();
                            block_meta_notifier
                                .notify_block_metadata(
                                    block.meta.parent_slot,
                                    todo_previous_blockhash.to_string().as_str(),
                                    block.slot,
                                    todo_latest_entry_blockhash.to_string().as_str(),
                                    &KeyedRewardsAndNumPartitions {
                                        keyed_rewards,
                                        num_partitions: None
                                    },
                                    Some(block.meta.blocktime as i64) ,
                                    block.meta.block_height,
                                    this_block_executed_transaction_count,
                                    this_block_entry_count,
                                );
                        }
                        todo_previous_blockhash = todo_latest_entry_blockhash;
                    }
                    node::Node::Subset(_subset) => {
                        // println!("___ Subset: {:?}", subset);
                    }
                    node::Node::Epoch(epoch) => {
                        println!("___ Epoch: {:?}", epoch);
                    }
                    node::Node::Rewards(rewards) => {
                        println!("___ Rewards: {:?}", node_with_cid.get_cid());
                        // println!("___ Next items: {:?}", rewards.data.next);

                        #[allow(clippy::overly_complex_bool_expr)]
                        if !rewards.is_complete() && false {
                            let reassembled = nodes.reassemble_dataframes(rewards.data.clone())?;
                            println!("___ reassembled: {:?}", reassembled.len());

                            let decompressed = utils::decompress_zstd(reassembled)?;

                            this_block_rewards = prost_011::Message::decode(decompressed.as_slice()).map_err(|err| {
                                Box::new(std::io::Error::new(
                                    std::io::ErrorKind::Other,
                                    std::format!("Error decoding rewards: {:?}", err),
                                ))
                            })?;
                        }
                    }
                    node::Node::DataFrame(_) => {
                        println!("___ DataFrame: {:?}", node_with_cid.get_cid());
                    }
                }
                Ok(())
            }).map_err(|err| {
                Box::new(std::io::Error::new(
                    std::io::ErrorKind::Other,
                    std::format!("Error processing node: {:?}", err),
                ))
            })?;
        }
    }
}

pub struct MessageAddressLoaderFromTxMeta {
    pub tx_meta: solana_transaction_status::TransactionStatusMeta,
}

impl MessageAddressLoaderFromTxMeta {
    pub fn new(tx_meta: solana_transaction_status::TransactionStatusMeta) -> Self {
        MessageAddressLoaderFromTxMeta { tx_meta }
    }
}

impl solana_sdk::message::AddressLoader for MessageAddressLoaderFromTxMeta {
    fn load_addresses(
        self,
        _lookups: &[solana_sdk::message::v0::MessageAddressTableLookup],
    ) -> Result<solana_sdk::message::v0::LoadedAddresses, solana_sdk::message::AddressLoaderError>
    {
        Ok(self.tx_meta.loaded_addresses.clone())
    }
}

// implement clone for MessageAddressLoaderFromTxMeta
impl Clone for MessageAddressLoaderFromTxMeta {
    fn clone(&self) -> Self {
        MessageAddressLoaderFromTxMeta {
            tx_meta: self.tx_meta.clone(),
        }
    }
}

pub fn get_program_ids(
    tx: &solana_sdk::transaction::VersionedTransaction,
) -> impl Iterator<Item = &solana_sdk::pubkey::Pubkey> + '_ {
    let message = &tx.message;
    let account_keys = message.static_account_keys();

    message
        .instructions()
        .iter()
        .map(|ix| ix.program_id(account_keys))
}

fn is_simple_vote_transaction(transaction: &solana_sdk::transaction::VersionedTransaction) -> bool {
    let signatures = transaction.signatures.clone();
    let is_legacy_message = matches!(
        transaction.version(),
        solana_sdk::transaction::TransactionVersion::Legacy(_)
    );

    let program_ids = get_program_ids(transaction);
    is_simple_vote_transaction_impl(&signatures, is_legacy_message, program_ids)
}

/// Simple vote transaction meets these conditions:
/// 1. has 1 or 2 signatures;
/// 2. is legacy message;
/// 3. has only one instruction;
/// 4. which must be Vote instruction;
#[inline]
pub fn is_simple_vote_transaction_impl<'a>(
    signatures: &[Signature],
    is_legacy_message: bool,
    mut instruction_programs: impl Iterator<Item = &'a solana_sdk::pubkey::Pubkey>,
) -> bool {
    signatures.len() < 3
        && is_legacy_message
        && instruction_programs
            .next()
            .xor(instruction_programs.next())
            .map(|program_id| program_id.to_string() == solana_sdk_ids::vote::ID.to_string())
            .unwrap_or(false)
}

use std::io::Read;

/// Opens either a local file or a streaming HTTP reader.
/// • For local files we return the `File` directly.
/// • For HTTP(S) URLs we issue a single GET and stream the body.
pub fn open_reader(path: &str) -> Result<Box<dyn Read + Send>, Box<dyn Error>> {
    if path.starts_with("http://") || path.starts_with("https://") {
        println!("Opening URL: {}", path);
        let resp = reqwest::blocking::get(path)?.error_for_status()?; // turn non-2xx into an error
        Ok(Box::new(resp))
    } else {
        println!("Opening file: {}", path);
        let file = std::fs::File::open(path)?;
        Ok(Box::new(file))
    }
}
