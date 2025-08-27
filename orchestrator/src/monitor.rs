use crate::{
    config::Config, epoch_state::EpochState, faithful_config::ConfigGenerator,
    storage::StorageManager,
};
use anyhow::Result;
use rand::seq::SliceRandom;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Instant;
use tokio::{
    sync::RwLock,
    time::{interval, Duration},
};
use tracing::{debug, error, info, warn};

#[derive(Debug, Clone)]
struct StorageHealth {
    storage_id: String,
    is_online: bool,
    last_check: Instant,
    consecutive_failures: u32,
    last_error: Option<String>,
}

impl StorageHealth {
    fn new(storage_id: String) -> Self {
        Self {
            storage_id,
            is_online: true,
            last_check: Instant::now(),
            consecutive_failures: 0,
            last_error: None,
        }
    }
}

/// Start the monitoring system that periodically checks storage health
pub async fn start_monitoring(
    config: Arc<Config>,
    storage_manager: Arc<StorageManager>,
    epoch_state: Arc<RwLock<EpochState>>,
) -> Result<()> {
    if !config.monitoring.enabled {
        info!("Storage monitoring is disabled");
        return Ok(());
    }

    let interval_duration = config.monitoring.parse_interval()?;
    let mut monitor_interval = interval(interval_duration);
    let mut storage_health: HashMap<String, StorageHealth> = HashMap::new();

    // Initialize health tracking for each storage
    for (idx, _) in config.storage.iter().enumerate() {
        let storage_id = format!("storage_{}", idx);
        storage_health.insert(storage_id.clone(), StorageHealth::new(storage_id));
    }

    info!(
        "Storage monitoring started with interval: {:?}",
        interval_duration
    );

    loop {
        monitor_interval.tick().await;

        info!("Starting periodic storage health check");

        // Check each storage endpoint individually
        for (idx, _storage) in config.storage.iter().enumerate() {
            let storage_id = format!("storage_{}", idx);

            // Perform random epoch validation for this storage
            match check_storage_with_random_epoch(&storage_manager, &epoch_state, &storage_id).await
            {
                Ok(is_healthy) => {
                    let health = storage_health.get_mut(&storage_id).unwrap();

                    if is_healthy {
                        if !health.is_online {
                            info!("Storage {} is back online", storage_id);
                        }
                        health.is_online = true;
                        health.consecutive_failures = 0;
                        health.last_error = None;
                    } else {
                        health.consecutive_failures += 1;
                        health.last_error = Some("Validation failed".to_string());

                        if health.consecutive_failures >= 3 {
                            warn!(
                                "Storage {} marked as offline after {} consecutive failures",
                                storage_id, health.consecutive_failures
                            );
                            health.is_online = false;

                            // Trigger full scan of failed storage
                            info!("Triggering full scan of storage {}", storage_id);
                            if let Err(e) =
                                trigger_storage_scan(&storage_manager, &epoch_state, &storage_id)
                                    .await
                            {
                                error!("Failed to scan storage {}: {}", storage_id, e);
                            }
                        }
                    }
                    health.last_check = Instant::now();
                }
                Err(e) => {
                    error!("Failed to check storage {}: {}", storage_id, e);
                    let health = storage_health.get_mut(&storage_id).unwrap();
                    health.is_online = false;
                    health.last_error = Some(e.to_string());
                    health.last_check = Instant::now();

                    // Trigger full scan on error
                    info!(
                        "Triggering full scan of storage {} due to error",
                        storage_id
                    );
                    if let Err(e) =
                        trigger_storage_scan(&storage_manager, &epoch_state, &storage_id).await
                    {
                        error!("Failed to scan storage {}: {}", storage_id, e);
                    }
                }
            }
        }

        // Handle any offline storages
        let offline_storages: Vec<String> = storage_health
            .values()
            .filter(|h| !h.is_online)
            .map(|h| h.storage_id.clone())
            .collect();

        if !offline_storages.is_empty() {
            warn!(
                "Found {} offline storage(s): {:?}",
                offline_storages.len(),
                offline_storages
            );

            // Regenerate affected configs
            if let Err(e) = handle_storage_failures(&config, &epoch_state, &offline_storages).await
            {
                error!("Failed to handle storage failures: {}", e);
            }
        }
    }
}

async fn check_storage_with_random_epoch(
    storage_manager: &StorageManager,
    epoch_state: &Arc<RwLock<EpochState>>,
    storage_id: &str,
) -> Result<bool> {
    // Get available epochs for this storage (CAR files)
    let epoch_state_guard = epoch_state.read().await;
    let available_epochs = epoch_state_guard.get_storage_epochs(storage_id);

    if available_epochs.is_empty() {
        debug!("Storage {} has no epochs available", storage_id);
        // Also check if this storage has any indexes
        let has_indexes = epoch_state_guard.storage_has_any_indexes(storage_id);
        if !has_indexes {
            debug!(
                "Storage {} has no data (no CAR files or indexes)",
                storage_id
            );
            return Ok(true); // Consider empty storage as "healthy"
        }

        // Storage only has indexes, validate one of them
        return validate_random_index(storage_manager, &epoch_state_guard, storage_id).await;
    }

    // Randomly select an epoch to validate
    let selected_epoch = {
        let mut rng = rand::thread_rng();
        available_epochs.choose(&mut rng).copied().unwrap_or(0)
    };

    info!(
        "Validating epoch {} CAR file in storage {}",
        selected_epoch, storage_id
    );

    // Check if we can access the epoch's CAR file
    if let Some(location) = epoch_state_guard.get_epoch_location(selected_epoch, storage_id) {
        // Try to validate the CAR file
        match storage_manager
            .validate_car_file(storage_id, &location.car_path)
            .await
        {
            Ok(true) => {
                debug!(
                    "CAR file validation successful for epoch {} in storage {}",
                    selected_epoch, storage_id
                );
                Ok(true)
            }
            Ok(false) => {
                debug!(
                    "CAR file validation failed for epoch {} in storage {}",
                    selected_epoch, storage_id
                );
                Ok(false)
            }
            Err(e) => {
                error!(
                    "Error validating epoch {} in storage {}: {}",
                    selected_epoch, storage_id, e
                );
                Err(e)
            }
        }
    } else {
        debug!(
            "Epoch {} not found in storage {}",
            selected_epoch, storage_id
        );
        Ok(false)
    }
}

async fn validate_random_index(
    storage_manager: &StorageManager,
    epoch_state: &EpochState,
    storage_id: &str,
) -> Result<bool> {
    // Get indexes available in this storage
    let available_indexes = epoch_state.get_storage_indexes(storage_id);

    if available_indexes.is_empty() {
        return Ok(true); // No indexes to validate
    }

    // Pick a random index to validate
    let index_to_validate = {
        let mut rng = rand::thread_rng();
        available_indexes.choose(&mut rng).cloned()
    };
    
    if let Some(index_info) = index_to_validate {
        info!(
            "Validating index {:?} for epoch {} in storage {}",
            index_info.index_type, index_info.epoch, storage_id
        );

        // Try to validate the index file exists
        match storage_manager
            .validate_index_file(storage_id, &index_info.path)
            .await
        {
            Ok(valid) => Ok(valid),
            Err(e) => {
                error!("Error validating index in storage {}: {}", storage_id, e);
                Err(e)
            }
        }
    } else {
        Ok(true)
    }
}

async fn trigger_storage_scan(
    storage_manager: &StorageManager,
    epoch_state: &Arc<RwLock<EpochState>>,
    storage_id: &str,
) -> Result<()> {
    info!("Performing full scan of storage: {}", storage_id);

    // Scan only the specific storage that failed
    let (car_results, index_results) = storage_manager.scan_single_storage(storage_id).await?;

    // Update epoch state with new scan results
    let mut epoch_state_guard = epoch_state.write().await;
    epoch_state_guard.update_storage_scan(storage_id, &car_results, &index_results);

    info!("Storage {} scan complete", storage_id);
    Ok(())
}

async fn handle_storage_failures(
    config: &Config,
    epoch_state: &Arc<RwLock<EpochState>>,
    offline_storages: &[String],
) -> Result<()> {
    // Only regenerate configs if output directory is configured
    if let Some(epoch_configs) = &config.epoch_configs {
        info!("Regenerating Old Faithful configurations due to storage failures");

        let epoch_state_guard = epoch_state.read().await;
        let affected_epochs = find_affected_epochs(&epoch_state_guard, offline_storages);

        if affected_epochs.is_empty() {
            info!("No epochs affected by storage failures");
            return Ok(());
        }

        info!(
            "Found {} epochs affected by storage failures",
            affected_epochs.len()
        );

        let config_generator = ConfigGenerator::new(&epoch_configs.output_dir);

        for epoch in affected_epochs {
            // Check if alternative storage is available
            let has_alternative =
                has_alternative_storage(&epoch_state_guard, epoch, offline_storages);

            if has_alternative {
                // Regenerate config to use alternative storage
                match config_generator
                    .generate_single_epoch_config(&*epoch_state_guard, epoch)
                    .await
                {
                    Ok(path) => {
                        info!(
                            "Regenerated config for epoch {} at {}",
                            epoch,
                            path.display()
                        );
                    }
                    Err(e) => {
                        error!("Failed to regenerate config for epoch {}: {}", epoch, e);
                    }
                }
            } else if config.monitoring.delete_on_total_failure {
                // Delete config if no alternatives and configured to do so
                match config_generator.delete_epoch_config(epoch).await {
                    Ok(_) => {
                        warn!(
                            "Deleted config for epoch {} (no alternative storage available)",
                            epoch
                        );
                    }
                    Err(e) => {
                        error!("Failed to delete config for epoch {}: {}", epoch, e);
                    }
                }
            } else {
                warn!(
                    "Epoch {} has no alternative storage but deletion is disabled",
                    epoch
                );
            }
        }

        // Regenerate combined config
        match config_generator
            .generate_combined_config(&*epoch_state_guard)
            .await
        {
            Ok(path) => {
                info!("Regenerated combined configuration at {}", path.display());
            }
            Err(e) => {
                error!("Failed to regenerate combined configuration: {}", e);
            }
        }
    }

    Ok(())
}

fn find_affected_epochs(epoch_state: &EpochState, offline_storages: &[String]) -> Vec<u64> {
    let mut affected = Vec::new();

    for epoch in epoch_state.get_all_epochs() {
        // Check if this epoch's primary storage is offline
        if let Some(best_location) = epoch_state.get_best_location(epoch) {
            if offline_storages.contains(&best_location.storage_id) {
                affected.push(epoch);
            }
        }
    }

    affected
}

fn has_alternative_storage(
    epoch_state: &EpochState,
    epoch: u64,
    offline_storages: &[String],
) -> bool {
    let locations = epoch_state.get_epoch_locations(epoch);

    // Check if there's at least one online storage with this epoch
    locations
        .iter()
        .any(|loc| !offline_storages.contains(&loc.storage_id))
}
