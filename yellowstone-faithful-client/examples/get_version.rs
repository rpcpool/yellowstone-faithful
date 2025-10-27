use clap::Parser;
use tracing_subscriber;
use yellowstone_faithful_client::{connect_with_config, GrpcConfig};

#[derive(Parser, Debug)]
#[command(author, version, about = "Get Old Faithful server version", long_about = None)]
struct Args {
    /// gRPC endpoint URL (e.g., http://localhost:8889 or https://api.example.com:443)
    #[arg(short, long)]
    endpoint: String,

    /// Authentication token (sent as x-token header)
    #[arg(short = 't', long)]
    x_token: Option<String>,
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

    println!("Connected! Fetching version information...\n");

    // Get version information
    let version_info = client.get_version().await?;

    // Display results
    println!("╔═══════════════════════════════════════╗");
    println!("║     Old Faithful Version Info         ║");
    println!("╚═══════════════════════════════════════╝");
    println!("  Version: {}", version_info.version);
    println!();

    Ok(())
}
