use solana_sdk::commitment_config::CommitmentConfig;
use solana_client::{ rpc_client::RpcClient, rpc_config::RpcBlockConfig };
use solana_transaction_status::{ TransactionDetails, UiTransactionEncoding };

fn main() {
    let faithful_rpc = RpcClient::new("https://rpc.old-faithful.net".to_string());

    let block = faithful_rpc.get_block_with_config(
        209520021,
        RpcBlockConfig {
            transaction_details: Some(TransactionDetails::Full),
            commitment: Some(CommitmentConfig::finalized()),
            max_supported_transaction_version: Some(0),
            encoding: Some(UiTransactionEncoding::Base64),
            rewards: Some(true),
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
}
