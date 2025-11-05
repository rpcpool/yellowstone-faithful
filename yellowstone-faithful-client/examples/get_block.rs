use {
    clap::Parser,
    yellowstone_faithful_client::{connect_with_config, GrpcConfig},
};

#[derive(Parser, Debug)]
#[command(author, version, about = "Get block by slot number", long_about = None)]
struct Args {
    /// gRPC endpoint URL
    #[arg(short, long)]
    endpoint: String,

    /// Authentication token (sent as x-token header)
    #[arg(short = 't', long)]
    x_token: Option<String>,

    /// Slot number to fetch
    #[arg(short, long)]
    slot: u64,
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

    println!("Connected! Fetching block at slot {}...\n", args.slot);

    // Get block
    let block = client.get_block(args.slot).await?;

    // Display results
    println!("╔═══════════════════════════════════════════════════════╗");
    println!("║                    Block Information                   ║");
    println!("╚═══════════════════════════════════════════════════════╝");
    println!("  Slot:              {}", block.slot);
    println!("  Parent Slot:       {}", block.parent_slot);
    println!("  Blockhash:         {}", block.blockhash);
    println!(
        "  Previous Hash:     {}",
        block
            .previous_blockhash
            .map(|h| h.to_string())
            .unwrap_or_else(|| "None (epoch boundary)".to_string())
    );
    println!("  Block Height:      {:?}", block.block_height);
    println!("  Block Time:        {:?}", block.block_time);
    println!("  Transactions:      {}", block.transaction_count());
    println!("  Num Partitions:    {:?}", block.num_partitions);

    if !block.transactions.is_empty() {
        println!(
            "\n  First {} transaction(s):",
            block.transactions.len().min(3)
        );
        for (i, tx) in block.transactions.iter().take(3).enumerate() {
            println!(
                "    {}. Transaction size: {} bytes",
                i + 1,
                tx.transaction.len()
            );
        }
    }

    println!();

    Ok(())
}
