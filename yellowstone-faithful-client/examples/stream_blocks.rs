use clap::Parser;
use futures::StreamExt;
use tracing_subscriber;
use yellowstone_faithful_client::{connect_with_config, GrpcConfig, StreamBlocksFilter};

#[derive(Parser, Debug)]
#[command(author, version, about = "Stream blocks from Old Faithful", long_about = None)]
struct Args {
    /// gRPC endpoint URL
    #[arg(short, long)]
    endpoint: String,

    /// Authentication token (sent as x-token header)
    #[arg(short = 't', long)]
    x_token: Option<String>,

    /// Start slot (inclusive)
    #[arg(long)]
    start_slot: u64,

    /// End slot (inclusive, optional - streams indefinitely if not set)
    #[arg(long)]
    end_slot: Option<u64>,

    /// Maximum number of blocks to fetch (optional, stops after this many)
    #[arg(short, long, default_value = "100")]
    limit: usize,

    /// Filter blocks by accounts (can be specified multiple times)
    #[arg(long = "account-include")]
    account_include: Vec<String>,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing
    tracing_subscriber::fmt::init();

    // Parse command-line arguments
    let args = Args::parse();

    println!("Connecting to Old Faithful gRPC at {}...", args.endpoint);

    // Create gRPC configuration
    let mut config = GrpcConfig::new(args.endpoint);
    if let Some(token) = args.x_token {
        config = config.with_token(token);
    }

    // Connect to the gRPC server
    let mut client = connect_with_config(config).await?;

    println!(
        "Connected! Streaming blocks from slot {} to {:?}...\n",
        args.start_slot, args.end_slot
    );

    // Prepare filter if accounts are specified
    let filter = if !args.account_include.is_empty() {
        println!("Filtering by accounts: {:?}\n", args.account_include);
        Some(StreamBlocksFilter {
            account_include: args.account_include,
        })
    } else {
        None
    };

    // Start streaming blocks
    let mut stream = client
        .stream_blocks(args.start_slot, args.end_slot, filter)
        .await?;

    let mut count = 0;
    let mut error_count = 0;

    println!("╔═══════════════════════════════════════════════════════╗");
    println!("║              Streaming Blocks (Ctrl+C to stop)        ║");
    println!("╚═══════════════════════════════════════════════════════╝\n");

    // Process stream
    while let Some(result) = stream.next().await {
        match result {
            Ok(block) => {
                count += 1;
                println!(
                    "Block #{}: slot={}, txs={}, time={:?}, hash={}",
                    count,
                    block.slot,
                    block.transaction_count(),
                    block.block_time,
                    block.blockhash
                );

                // Stop after limit
                if count >= args.limit {
                    println!("\n✓ Reached limit of {} blocks", args.limit);
                    break;
                }
            }
            Err(e) => {
                error_count += 1;
                eprintln!("⚠️  Error receiving block #{}: {}", error_count, e);

                // Continue on parsing errors, break on connection errors
                let err_msg = format!("{}", e);
                if err_msg.contains("Invalid block data") || err_msg.contains("blockhash") {
                    eprintln!("   → Skipping invalid block, continuing...\n");
                    continue;
                } else {
                    eprintln!("   → Fatal error, stopping stream\n");
                    break;
                }
            }
        }
    }

    // Summary
    println!("\n{}", "=".repeat(60));
    println!("Stream Summary:");
    println!("  ✓ Successfully processed: {} blocks", count);
    println!("  ✗ Errors encountered:     {}", error_count);
    println!("{}", "=".repeat(60));

    Ok(())
}
