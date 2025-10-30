use {
    clap::Parser,
    futures::StreamExt,
    tracing_subscriber,
    yellowstone_faithful_client::{connect_with_config, GrpcConfig, StreamTransactionsFilter},
};

#[derive(Parser, Debug)]
#[command(author, version, about = "Stream transactions from Old Faithful", long_about = None)]
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

    /// Maximum number of transactions to fetch
    #[arg(short, long, default_value = "100")]
    limit: usize,

    /// Exclude vote transactions
    #[arg(long = "no-vote")]
    no_vote: bool,

    /// Only include failed transactions
    #[arg(long = "failed-only")]
    failed_only: bool,

    /// Filter transactions by accounts to include (can be specified multiple times)
    #[arg(long = "account-include")]
    account_include: Vec<String>,

    /// Filter transactions by accounts to exclude (can be specified multiple times)
    #[arg(long = "account-exclude")]
    account_exclude: Vec<String>,

    /// Filter transactions requiring specific accounts (can be specified multiple times)
    #[arg(long = "account-required")]
    account_required: Vec<String>,
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
        "Connected! Streaming transactions from slot {} to {:?}...\n",
        args.start_slot, args.end_slot
    );

    // Build filter
    let has_filter = args.no_vote
        || args.failed_only
        || !args.account_include.is_empty()
        || !args.account_exclude.is_empty()
        || !args.account_required.is_empty();

    let filter = if has_filter {
        println!("Applied filters:");
        if args.no_vote {
            println!("  • Excluding vote transactions");
        }
        if args.failed_only {
            println!("  • Only failed transactions");
        }
        if !args.account_include.is_empty() {
            println!("  • Include accounts: {:?}", args.account_include);
        }
        if !args.account_exclude.is_empty() {
            println!("  • Exclude accounts: {:?}", args.account_exclude);
        }
        if !args.account_required.is_empty() {
            println!("  • Required accounts: {:?}", args.account_required);
        }
        println!();

        Some(StreamTransactionsFilter {
            vote: if args.no_vote { Some(false) } else { None },
            failed: if args.failed_only { Some(true) } else { None },
            account_include: args.account_include,
            account_exclude: args.account_exclude,
            account_required: args.account_required,
        })
    } else {
        None
    };

    // Start streaming transactions
    let mut stream = client
        .stream_transactions(args.start_slot, args.end_slot, filter)
        .await?;

    let mut count = 0;
    let mut error_count = 0;

    println!("╔═══════════════════════════════════════════════════════╗");
    println!("║          Streaming Transactions (Ctrl+C to stop)      ║");
    println!("╚═══════════════════════════════════════════════════════╝\n");

    // Process stream
    while let Some(result) = stream.next().await {
        match result {
            Ok(tx) => {
                count += 1;
                println!(
                    "Transaction #{}: slot={}, size={} bytes, index={:?}",
                    count,
                    tx.slot,
                    tx.transaction.transaction.len(),
                    tx.index
                );

                // Stop after limit
                if count >= args.limit {
                    println!("\n✓ Reached limit of {} transactions", args.limit);
                    break;
                }
            }
            Err(e) => {
                error_count += 1;
                eprintln!("⚠️  Error receiving transaction #{}: {}", error_count, e);

                // Continue on parsing errors, break on connection errors
                let err_msg = format!("{}", e);
                if err_msg.contains("Invalid") || err_msg.contains("parse") {
                    eprintln!("   → Skipping invalid transaction, continuing...\n");
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
    println!("  ✓ Successfully processed: {} transactions", count);
    println!("  ✗ Errors encountered:     {}", error_count);
    println!("{}", "=".repeat(60));

    Ok(())
}
