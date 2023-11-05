use solana_sdk::{commitment_config::CommitmentConfig, signature::Signature};
use solana_client::{ rpc_client::RpcClient, rpc_config::RpcBlockConfig, rpc_config::RpcTransactionConfig };
use solana_transaction_status::{ TransactionDetails, UiTransactionEncoding };

use std::str::FromStr;


fn main() {
    let faithful_rpc = RpcClient::new("https://rpc.old-faithful.net".to_string());

    let block = faithful_rpc.get_block_with_config(
        209520021,
        RpcBlockConfig {
            transaction_details: Some(TransactionDetails::None),
            commitment: Some(CommitmentConfig::finalized()),
            max_supported_transaction_version: Some(0),
            encoding: Some(UiTransactionEncoding::Base64),
            rewards: Some(false),
        },
    );
    println!("got reply from faithful");
    match block {
        Ok(block) => {
            println!("blockhash : {}", block.blockhash);
        },
        Err(e) => {
            println!("Err: {}", e);
        },
    };

    let sig_s = "3qEUUW9fKaZpECvJ87QfZMyVMQjR1GBKnuCDqJMCgxw1sCzrWSU6q5ydEiX1JEJPbQDGaNoxULxmCW6f4mAnNRo2";
    let sig = Signature::from_str(&sig_s).unwrap();
    let transaction = faithful_rpc.get_transaction_with_config(
      &sig,
      RpcTransactionConfig {
        encoding: Some(UiTransactionEncoding::Base64),
        commitment: Some(CommitmentConfig::finalized()),
        max_supported_transaction_version: Some(0),
      },
      );
  match transaction {
      Ok(transaction) => {
        println!("{:?}", transaction);
      },
      Err(e) => {
        println!("Err: {:?}", e);
      },
    };
      
}
