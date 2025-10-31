use {
    clap::Parser,
    yellowstone_faithful_client::{connect_with_config, GrpcConfig},
};

#[derive(Parser, Debug)]
#[command(author, version, about = "Get block time for a slot", long_about = None)]
struct Args {
    /// gRPC endpoint URL
    #[arg(short, long)]
    endpoint: String,

    /// Authentication token (sent as x-token header)
    #[arg(short = 't', long)]
    x_token: Option<String>,

    /// Slot number to fetch block time for
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

    println!("Connected! Fetching block time for slot {}...\n", args.slot);

    // Get block time
    let block_time = client.get_block_time(args.slot).await?;

    // Display results
    println!("╔═══════════════════════════════════════╗");
    println!("║          Block Time Info              ║");
    println!("╚═══════════════════════════════════════╝");
    println!("  Slot:       {}", args.slot);
    println!("  Block Time: {}", block_time.block_time);
    // Convert Unix timestamp to human-readable format
    if let Some(datetime) = chrono::DateTime::from_timestamp(block_time.block_time, 0) {
        println!("  Date/Time:  {}", datetime.format("%Y-%m-%d %H:%M:%S UTC"));
    }

    println!();

    Ok(())
}
