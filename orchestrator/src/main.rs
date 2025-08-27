mod api;
mod cache;
mod car_report;
mod config;
mod epoch_state;
mod faithful_config;
mod storage;

use anyhow::Result;
use cache::DiskCache;
use clap::Parser;
use epoch_state::EpochState;
use std::path::PathBuf;
use std::sync::Arc;
use storage::StorageManager;
use tokio::sync::RwLock;
use tracing::{info, warn};

#[derive(Parser, Debug)]
#[clap(name = "orchestrator")]
#[clap(about = "Old Faithful Orchestrator - Manages storage locations for CAR files and indexes")]
#[clap(version)]
struct Args {
    /// Path to the configuration file
    #[clap(short, long, value_name = "FILE")]
    config: PathBuf,

    /// Increase logging verbosity
    #[clap(short, long, action = clap::ArgAction::Count)]
    verbose: u8,

    /// Disable cache even if enabled in config
    #[clap(long)]
    no_cache: bool,

    /// Clear cache before starting
    #[clap(long)]
    clear_cache: bool,

    /// Override cache directory from config
    #[clap(long, value_name = "DIR")]
    cache_dir: Option<PathBuf>,

    /// Print cache statistics after operation
    #[clap(long)]
    cache_stats: bool,
    
    /// HTTP API port (default: 8080)
    #[clap(short, long, default_value = "8080")]
    port: u16,
}

fn init_logging(verbosity: u8) {
    let log_level = match verbosity {
        0 => tracing::Level::INFO,
        1 => tracing::Level::DEBUG,
        _ => tracing::Level::TRACE,
    };

    tracing_subscriber::fmt()
        .with_max_level(log_level)
        .with_target(false)
        .with_thread_ids(false)
        .with_file(verbosity > 1)
        .with_line_number(verbosity > 1)
        .init();
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();
    
    init_logging(args.verbose);
    
    info!("Starting Old Faithful Orchestrator");
    info!("Loading configuration from: {}", args.config.display());
    
    // Load and validate configuration
    let mut config = config::Config::from_file(&args.config)?;
    config.validate()?;
    
    // Override cache settings from CLI arguments
    if args.no_cache {
        info!("Cache disabled via --no-cache flag");
        config.cache.enabled = false;
    }
    
    if let Some(cache_dir) = args.cache_dir {
        info!("Overriding cache directory to: {}", cache_dir.display());
        config.cache.directory = cache_dir.to_string_lossy().to_string();
    }
    
    info!("Configuration loaded successfully");
    info!("Found {} storage backend(s)", config.storage.len());
    
    // Clear cache if requested
    if args.clear_cache && config.cache.enabled {
        info!("Clearing cache directory: {}", config.cache.directory);
        if let Err(e) = std::fs::remove_dir_all(&config.cache.directory) {
            if e.kind() != std::io::ErrorKind::NotFound {
                warn!("Failed to clear cache directory: {}", e);
            }
        }
    }
    
    // Initialize cache if enabled
    let _cache: Option<DiskCache<Vec<u8>>> = if config.cache.enabled {
        info!("Cache enabled - initializing disk cache");
        info!("Cache directory: {}", config.cache.directory);
        info!("Cache TTL: {}", config.cache.ttl);
        
        // Parse TTL to validate it
        let ttl_duration = config.cache.parse_ttl()?;
        info!("Parsed cache TTL: {:?}", ttl_duration);
        
        // Create cache configuration
        let cache_config = cache::CacheConfig {
            default_ttl: Some(ttl_duration),
            ..Default::default()
        };
        
        // Initialize the cache
        match DiskCache::new(&config.cache.directory, cache_config).await {
            Ok(disk_cache) => {
                info!("Cache initialized successfully");
                Some(disk_cache)
            }
            Err(e) => {
                warn!("Failed to initialize cache: {}", e);
                warn!("Continuing without cache");
                None
            }
        }
    } else {
        info!("Cache disabled");
        None
    };
    
    // Log storage configurations
    for (i, storage) in config.storage.iter().enumerate() {
        match storage {
            config::StorageConfig::Local(local) => {
                info!("Storage #{}: Local filesystem at {}", i + 1, local.path);
            }
            config::StorageConfig::Http(http) => {
                info!("Storage #{}: HTTP endpoint at {}", i + 1, http.url);
            }
        }
    }
    
    // Create storage manager with cache support if enabled
    let storage_manager = Arc::new(if config.cache.enabled {
        StorageManager::from_config_with_cache(&config.storage, &config.cache).await?
    } else {
        StorageManager::from_config(&config.storage).await?
    });
    
    // Initialize empty epoch state that will be populated during scanning
    let epoch_state = Arc::new(RwLock::new(EpochState::new()));
    
    // Start the HTTP API server immediately so it's available during startup
    let config_arc = Arc::new(config.clone());
    let api_handle = tokio::spawn(api::start_server(
        config_arc.clone(),
        storage_manager.clone(),
        epoch_state.clone(),
        args.port,
    ));
    
    info!("HTTP API server starting on http://localhost:{}", args.port);
    info!("You can monitor startup progress at http://localhost:{}/status", args.port);
    
    // Give the API server a moment to start
    tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
    
    // Now perform the storage scanning
    info!("Scanning storage locations for CAR files and indexes...");
    let (car_scan_results, index_scan_results) = storage_manager.scan_all_files_optimized().await?;
    
    // Log CAR file scan results summary
    let mut total_car_files = 0;
    for (storage_id, car_files) in &car_scan_results {
        if car_files.is_empty() {
            warn!("No CAR files found in storage: {}", storage_id);
        } else {
            // Don't log here since the backend already logged it
            total_car_files += car_files.len();
        }
    }
    
    if total_car_files == 0 {
        warn!("No CAR files found in any storage location");
    } else {
        info!("Storage validation completed successfully");
    }
    
    // Update the epoch state with scan results
    info!("Building epoch availability state...");
    {
        let mut epoch_state_guard = epoch_state.write().await;
        *epoch_state_guard = EpochState::from_scan_results(&car_scan_results, &index_scan_results);
    }
    
    // Display summary
    {
        let epoch_state = epoch_state.read().await;
        info!("\n{}", epoch_state.summary());
    }
    
    // Show some detailed information
    let all_epochs = {
        let epoch_state = epoch_state.read().await;
        epoch_state.get_all_epochs()
    };
    if !all_epochs.is_empty() {
        info!("Epoch range: {} to {}", all_epochs.first().unwrap(), all_epochs.last().unwrap());
        
        let epoch_state_guard = epoch_state.read().await;
        
        // Show epochs with single source (potential risk)
        let single_source = epoch_state_guard.find_single_source_epochs();
        if !single_source.is_empty() {
            warn!("Epochs with only single source (no redundancy): {:?}", 
                  &single_source[..single_source.len().min(10)]);
            if single_source.len() > 10 {
                warn!("... and {} more single-source epochs", single_source.len() - 10);
            }
        }
        
        // Show epochs with redundancy
        let redundant = epoch_state_guard.find_redundant_epochs();
        if !redundant.is_empty() {
            info!("Epochs with redundancy: {} epochs have multiple sources", redundant.len());
        }
        
        // Check for incomplete epochs (missing indexes)
        let incomplete = epoch_state_guard.find_incomplete_epochs();
        if !incomplete.is_empty() {
            warn!("Epochs missing required indexes: {:?}", 
                  &incomplete[..incomplete.len().min(10)]);
            if incomplete.len() > 10 {
                warn!("... and {} more incomplete epochs", incomplete.len() - 10);
            }
        } else {
            info!("All epochs have required indexes!");
        }
        
        // Example: Show best location for first few epochs
        for epoch in all_epochs.iter().take(3) {
            if let Some(best_location) = epoch_state_guard.get_best_location(*epoch) {
                let complete = if epoch_state_guard.is_epoch_complete(*epoch) { "✓" } else { "✗" };
                if let Some(cid) = epoch_state_guard.get_epoch_cid(*epoch) {
                    info!("Epoch {} [{}]: CID {} - best source is {} at {}", 
                          epoch, complete, cid, best_location.storage_id, best_location.car_path);
                } else {
                    info!("Epoch {} [{}]: best source is {} at {}", 
                          epoch, complete, best_location.storage_id, best_location.car_path);
                }
            }
        }
    } else {
        warn!("No epochs found in any storage location");
    }
    
    // Generate Old Faithful configuration files if output directory is configured
    if let Some(epoch_configs) = &config.epoch_configs {
        info!("Generating Old Faithful configuration files...");
        info!("Output directory: {}", epoch_configs.output_dir);
        
        let config_generator = faithful_config::ConfigGenerator::new(&epoch_configs.output_dir);
        
        // Generate individual configuration files for each complete epoch
        let epoch_state_guard = epoch_state.read().await;
        match config_generator.generate_configs(&*epoch_state_guard).await {
            Ok(generated_files) => {
                info!("Successfully generated {} configuration files", generated_files.len());
                
                // Also generate a combined configuration file
                match config_generator.generate_combined_config(&*epoch_state_guard).await {
                    Ok(combined_file) => {
                        info!("Generated combined configuration: {}", combined_file.display());
                    }
                    Err(e) => {
                        warn!("Failed to generate combined configuration: {}", e);
                    }
                }
            }
            Err(e) => {
                warn!("Failed to generate configuration files: {}", e);
            }
        }
    } else {
        info!("No epoch_configs.output_dir configured - skipping configuration file generation");
        info!("To generate Old Faithful configuration files, add the following to your config:");
        info!("  [epoch_configs]");
        info!("  output_dir = \"./configs\"");
    }
    
    info!("Orchestrator startup complete.");
    info!("HTTP API available at http://localhost:{}", args.port);
    info!("Press Ctrl+C to exit.");
    
    // Wait for shutdown signal
    tokio::select! {
        _ = tokio::signal::ctrl_c() => {
            info!("Received shutdown signal");
        }
        result = api_handle => {
            if let Err(e) = result {
                warn!("API server error: {}", e);
            }
        }
    }
    
    info!("Shutting down orchestrator");
    Ok(())
}
