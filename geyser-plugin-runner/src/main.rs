use {
    crossbeam_channel::unbounded,
    demo_rust_ipld_car::{node, utils},
    solana_message::VersionedMessage,
    solana_pubkey::Pubkey,
    solana_reward_info::{RewardInfo, RewardType},
    solana_rpc::optimistically_confirmed_bank_tracker::SlotNotification,
    solana_runtime::bank::KeyedRewardsAndNumPartitions,
    solana_transaction_error::AddressLoaderError,
    std::{convert::TryInto, env::args, error::Error, io::BufReader, str::FromStr},
};

fn main() -> Result<(), Box<dyn Error>> {
    let file_path = args().nth(1).expect("no file given");
    let _started_at = std::time::Instant::now();
    let file = std::fs::File::open(file_path)?;
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
                // new parameter `geyser_plugin_always_enabled`
                // Subscribe on all types of notifiactions, even if no config files are passed
                false,
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

        let mut todo_previous_blockhash = solana_hash::Hash::default();
        let mut todo_latest_entry_blockhash = solana_hash::Hash::default();
        loop {
            let nodes = reader.read_until_block()?;
            // println!("Nodes: {:?}", nodes.get_cids());
            let block = nodes.get_block()?;
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
                                nodes.reassemble_dataframes(transaction.metadata.clone())?;

                            let decompressed = utils::decompress_zstd(reassembled_metadata.clone())?;

                            let metadata: solana_storage_proto::convert::generated::TransactionStatusMeta =
                                prost_011::Message::decode(decompressed.as_slice()).map_err(|err| {
                                    Box::new(std::io::Error::new(
                                        std::io::ErrorKind::Other,
                                        std::format!("Error decoding metadata: {:?}", err),
                                    ))
                                })?;


                            let as_native_metadata: solana_transaction_status::TransactionStatusMeta =
                                metadata.try_into()?;

                           {
                                let message_hash = parsed.verify_and_hash_message()?;

                                // Simple vote transaction meets these conditions:
                                // 1. has 1 or 2 signatures;
                                // 2. is legacy message;
                                // 3. has only one instruction;
                                // 4. which must be Vote instruction;
                                let mut is_vote = false;
                                if let VersionedMessage::Legacy(legacy_message) = parsed.clone().message {
                                    let instructions_len = legacy_message.instructions.len();
                                    if instructions_len == 1 {
                                            let program_id_index = legacy_message.instructions[0].program_id_index as usize;
                                            if let Some(program_id) = legacy_message.account_keys.get(program_id_index) {
                                                if program_id == &Pubkey::from_str_const("Vote111111111111111111111111111111111111111") {
                                                    is_vote = true;
                                                }
                                            }
                                    }
                                }

                                transaction_notifier
                                        .notify_transaction(
                                            block.slot,
                                            transaction.index.unwrap() as usize,
                                            parsed.signatures.first().expect("A transaction should contains at least one signature"),
                                            &message_hash,
                                            is_vote,
                                            &as_native_metadata,
                                            &parsed,
                                        );
                            }
                        }

                        // if parsed.version()
                        //     == solana_transaction::versioned::TransactionVersion::Number(0)
                        // {
                        //     return Ok(());
                        // }
                    }
                    node::Node::Entry(_entry) => {
                        todo_latest_entry_blockhash = solana_hash::Hash::from(_entry.hash.to_bytes());
                        this_block_executed_transaction_count += _entry.transactions.len() as u64;
                        this_block_entry_count += 1;
                        if entry_notifier_maybe.is_none() {
                            return Ok(());
                        }
                        let entry_notifier = entry_notifier_maybe.as_ref().unwrap();
                        // println!("___ Entry: {:?}", entry);
                        let entry_summary=solana_entry::entry::EntrySummary {
                            num_hashes: _entry.num_hashes,
                            hash: solana_hash::Hash::from(_entry.hash.to_bytes()),
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
                                    keyed_rewards.push((solana_pubkey::Pubkey::from_str(&this_block_reward.pubkey)?, reward));
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

impl solana_message::AddressLoader for MessageAddressLoaderFromTxMeta {
    fn load_addresses(
        self,
        _lookups: &[solana_message::v0::MessageAddressTableLookup],
    ) -> Result<solana_message::v0::LoadedAddresses, AddressLoaderError> {
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
