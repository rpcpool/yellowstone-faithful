use {
    clap::Parser,
    tracing_subscriber,
    yellowstone_faithful_client::{connect_with_config, GrpcConfig},
};

#[derive(Parser, Debug)]
#[command(author, version, about = "Get transaction by signature", long_about = None)]
struct Args {
    /// gRPC endpoint URL
    #[arg(short, long)]
    endpoint: String,

    /// Authentication token (sent as x-token header)
    #[arg(short = 't', long)]
    x_token: Option<String>,

    /// Transaction signature (base58 encoded)
    #[arg(short, long)]
    signature: String,
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

    println!("Connected! Fetching transaction...\n");

    // Decode signature from base58
    let signature_bytes = bs58::decode(&args.signature)
        .into_vec()
        .map_err(|e| format!("Failed to decode signature: {}", e))?;

    // Get transaction
    let tx_with_context = client.get_transaction(&signature_bytes).await?;

    // Display results
    println!("╔═══════════════════════════════════════════════════════╗");
    println!("║              Transaction Information                   ║");
    println!("╚═══════════════════════════════════════════════════════╝");
    println!("  Signature:     {}", args.signature);
    println!("  Slot:          {}", tx_with_context.slot);
    println!("  Block Time:    {:?}", tx_with_context.block_time);
    if let Some(index) = tx_with_context.index {
        println!("  Index:         {} (position in block)", index);
    }

    println!();

    Ok(())
}
