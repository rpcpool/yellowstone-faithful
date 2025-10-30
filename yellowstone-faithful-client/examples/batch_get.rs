use {
    clap::Parser,
    futures::StreamExt,
    tracing_subscriber,
    yellowstone_faithful_client::{
        connect_with_config,
        grpc::generated::{get_request, BlockRequest, GetRequest},
        GrpcConfig,
    },
};

#[derive(Parser, Debug)]
#[command(author, version, about = "Batch get multiple blocks efficiently", long_about = None)]
struct Args {
    /// gRPC endpoint URL
    #[arg(short, long)]
    endpoint: String,

    /// Authentication token (sent as x-token header)
    #[arg(short = 't', long)]
    x_token: Option<String>,

    /// Comma-separated list of slot numbers to fetch (e.g., "100,101,102")
    #[arg(long, value_delimiter = ',')]
    slots: Vec<u64>,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize tracing
    tracing_subscriber::fmt::init();

    // Parse command-line arguments
    let args = Args::parse();

    if args.slots.is_empty() {
        eprintln!("Error: No slots specified. Use --slots 100,101,102");
        std::process::exit(1);
    }

    println!("Connecting to Old Faithful gRPC at {}...", args.endpoint);

    // Create gRPC configuration
    let mut config = GrpcConfig::new(args.endpoint);
    if let Some(token) = args.x_token {
        config = config.with_token(token);
    }

    // Connect to the gRPC server
    let mut client = connect_with_config(config).await?;

    println!("Connected! Batch fetching {} blocks...\n", args.slots.len());

    // Build batch requests
    let requests: Vec<GetRequest> = args
        .slots
        .iter()
        .enumerate()
        .map(|(i, &slot)| GetRequest {
            id: i as u64,
            request: Some(get_request::Request::Block(BlockRequest { slot })),
        })
        .collect();

    println!("Requesting slots: {:?}\n", args.slots);

    // Execute batch get
    let mut stream = client.batch_get(requests).await?;

    let mut count = 0;
    let mut error_count = 0;

    println!("╔═══════════════════════════════════════════════════════╗");
    println!("║              Batch Get Results                         ║");
    println!("╚═══════════════════════════════════════════════════════╝\n");

    // Process responses
    while let Some(result) = stream.next().await {
        match result {
            Ok(response) => match response.response {
                Some(get_response) => {
                    use yellowstone_faithful_client::grpc::generated::get_response::Response;
                    match get_response {
                        Response::Block(block_response) => {
                            count += 1;
                            println!(
                                "Response #{} (id={}): Block at slot {} - {} transactions",
                                count,
                                response.id,
                                block_response.slot,
                                block_response.transactions.len()
                            );
                        }
                        Response::Error(err) => {
                            error_count += 1;
                            eprintln!(
                                "Response #{} (id={}): Error - {:?}: {}",
                                count + error_count,
                                response.id,
                                err.code,
                                err.message
                            );
                        }
                        _ => {
                            println!(
                                "Response #{} (id={}): Other response type",
                                count + error_count + 1,
                                response.id
                            );
                        }
                    }
                }
                None => {
                    eprintln!("Received empty response for id={}", response.id);
                }
            },
            Err(e) => {
                error_count += 1;
                eprintln!("⚠️  Stream error: {}", e);
                break;
            }
        }
    }

    // Summary
    println!("\n{}", "=".repeat(60));
    println!("Batch Get Summary:");
    println!("  ✓ Successful responses: {}", count);
    println!("  ✗ Errors:              {}", error_count);
    println!("  📊 Total requested:     {}", args.slots.len());
    println!("{}", "=".repeat(60));

    Ok(())
}
