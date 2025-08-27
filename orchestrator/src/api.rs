use anyhow::Result;
use axum::{
    extract::State,
    http::StatusCode,
    response::Json,
    routing::get,
    Router,
};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use std::time::Instant;
use tokio::sync::RwLock;
use tower_http::cors::CorsLayer;
use tracing::info;

use crate::config::Config;
use crate::epoch_state::EpochState;
use crate::storage::{IndexType, StorageManager};

#[derive(Clone)]
pub struct AppState {
    pub config: Arc<Config>,
    pub storage_manager: Arc<StorageManager>,
    pub epoch_state: Arc<RwLock<EpochState>>,
    pub start_time: Instant,
}

#[derive(Serialize, Deserialize)]
pub struct ApiStatus {
    pub version: String,
    pub uptime_seconds: u64,
    pub uptime_ms: u128,  // Add milliseconds for debugging
    pub storage_count: usize,
    pub epoch_count: usize,
}

#[derive(Serialize, Deserialize)]
pub struct StorageStatus {
    pub id: String,
    pub storage_type: String,
    pub url: Option<String>,
    pub path: Option<String>,
    pub epoch_count: usize,
    pub epochs: Vec<u64>,
}

#[derive(Serialize, Deserialize)]
pub struct EpochStatus {
    pub epoch: u64,
    pub cid: Option<String>,
    pub is_complete: bool,
    pub car_locations: Vec<String>,
    pub missing_indexes: Vec<String>,
    pub size_bytes: Option<u64>,
}

#[derive(Serialize, Deserialize)]
pub struct OverviewResponse {
    pub status: ApiStatus,
    pub storages: Vec<StorageStatus>,
    pub summary: EpochSummary,
}

#[derive(Serialize, Deserialize)]
pub struct EpochSummary {
    pub total_epochs: usize,
    pub complete_epochs: usize,
    pub incomplete_epochs: usize,
    pub single_source_epochs: usize,
    pub redundant_epochs: usize,
    pub epoch_range: Option<(u64, u64)>,
}

pub fn create_router(state: AppState) -> Router {
    Router::new()
        .route("/", get(root_handler))
        .route("/status", get(status_handler))
        .route("/storages", get(storages_handler))
        .route("/epochs", get(epochs_handler))
        .route("/epochs/:epoch", get(epoch_detail_handler))
        .route("/overview", get(overview_handler))
        .layer(CorsLayer::permissive())
        .with_state(state)
}

async fn root_handler() -> Json<serde_json::Value> {
    Json(serde_json::json!({
        "service": "Old Faithful Orchestrator",
        "version": env!("CARGO_PKG_VERSION"),
        "endpoints": {
            "/": "This help message",
            "/status": "Service status",
            "/storages": "List all configured storage backends",
            "/epochs": "List all epochs and their status",
            "/epochs/{epoch}": "Get detailed status for a specific epoch",
            "/overview": "Get complete overview of the system"
        }
    }))
}

async fn status_handler(State(state): State<AppState>) -> Json<ApiStatus> {
    let epoch_state = state.epoch_state.read().await;
    let all_epochs = epoch_state.get_all_epochs();
    let elapsed = state.start_time.elapsed();
    let uptime_secs = elapsed.as_secs();
    let uptime_ms = elapsed.as_millis();
    
    Json(ApiStatus {
        version: env!("CARGO_PKG_VERSION").to_string(),
        uptime_seconds: uptime_secs,
        uptime_ms: uptime_ms,
        storage_count: state.config.storage.len(),
        epoch_count: all_epochs.len(),
    })
}

async fn storages_handler(State(state): State<AppState>) -> Result<Json<Vec<StorageStatus>>, StatusCode> {
    let epoch_state = state.epoch_state.read().await;
    let mut storages = Vec::new();
    
    for storage_config in &state.config.storage {
        let (id, storage_type, url, path) = match storage_config {
            crate::config::StorageConfig::Local(local) => {
                let id = format!("local:{}", local.path);
                (id.clone(), "local".to_string(), None, Some(local.path.clone()))
            }
            crate::config::StorageConfig::Http(http) => {
                let id = format!("http:{}", http.url);
                (id.clone(), "http".to_string(), Some(http.url.clone()), None)
            }
        };
        
        // Get epochs from the cached epoch state instead of triggering a scan
        let epochs: Vec<u64> = epoch_state
            .get_storage_epochs(&id)
            .map(|epoch_set| {
                let mut epochs: Vec<u64> = epoch_set.iter().copied().collect();
                epochs.sort();
                epochs
            })
            .unwrap_or_default();
        
        storages.push(StorageStatus {
            id: id.clone(),
            storage_type,
            url,
            path,
            epoch_count: epochs.len(),
            epochs,
        });
    }
    
    Ok(Json(storages))
}

async fn epochs_handler(State(state): State<AppState>) -> Json<Vec<EpochStatus>> {
    let epoch_state = state.epoch_state.read().await;
    let all_epochs = epoch_state.get_all_epochs();
    
    let mut epochs = Vec::new();
    for epoch in all_epochs {
        let cid = epoch_state.get_epoch_cid(epoch);
        let is_complete = epoch_state.is_epoch_complete(epoch);
        let locations = epoch_state.get_epoch_locations(epoch);
        let car_locations: Vec<String> = locations
            .map(|locs| locs.iter().map(|l| l.storage_id.clone()).collect())
            .unwrap_or_default();
        
        let missing_indexes = if !is_complete {
            let required_indexes = vec![IndexType::SlotToCid, IndexType::SigToCid, IndexType::CidToOffsetAndSize, IndexType::SigExists];
            let available_indexes = epoch_state.epoch_indexes.get(&epoch)
                .map(|indexes| indexes.iter().map(|i| i.index_type.clone()).collect::<Vec<_>>())
                .unwrap_or_default();
            
            required_indexes.into_iter()
                .filter(|idx| !available_indexes.contains(idx))
                .map(|idx| format!("{:?}", idx))
                .collect()
        } else {
            Vec::new()
        };
        
        let size_bytes = None; // Size is not stored in EpochLocation
        
        epochs.push(EpochStatus {
            epoch,
            cid,
            is_complete,
            car_locations,
            missing_indexes,
            size_bytes,
        });
    }
    
    Json(epochs)
}

async fn epoch_detail_handler(
    State(state): State<AppState>,
    axum::extract::Path(epoch): axum::extract::Path<u64>,
) -> Result<Json<EpochStatus>, StatusCode> {
    let epoch_state = state.epoch_state.read().await;
    
    let cid = epoch_state.get_epoch_cid(epoch);
    let is_complete = epoch_state.is_epoch_complete(epoch);
    let locations = epoch_state.get_epoch_locations(epoch);
    
    if locations.is_none() || locations.unwrap().is_empty() {
        return Err(StatusCode::NOT_FOUND);
    }
    
    let car_locations: Vec<String> = locations
        .map(|locs| locs.iter().map(|l| l.storage_id.clone()).collect())
        .unwrap_or_default();
    
    let missing_indexes = if !is_complete {
        let required_indexes = vec![IndexType::SlotToCid, IndexType::SigToCid, IndexType::CidToOffsetAndSize, IndexType::SigExists];
        let available_indexes = epoch_state.epoch_indexes.get(&epoch)
            .map(|indexes| indexes.iter().map(|i| i.index_type.clone()).collect::<Vec<_>>())
            .unwrap_or_default();
        
        required_indexes.into_iter()
            .filter(|idx| !available_indexes.contains(idx))
            .map(|idx| format!("{:?}", idx))
            .collect()
    } else {
        Vec::new()
    };
    
    let size_bytes = None; // Size is not stored in EpochLocation
    
    Ok(Json(EpochStatus {
        epoch,
        cid,
        is_complete,
        car_locations,
        missing_indexes,
        size_bytes,
    }))
}

async fn overview_handler(State(state): State<AppState>) -> Result<Json<OverviewResponse>, StatusCode> {
    let epoch_state = state.epoch_state.read().await;
    let all_epochs = epoch_state.get_all_epochs();
    
    let complete_epochs = all_epochs.iter()
        .filter(|&&e| epoch_state.is_epoch_complete(e))
        .count();
    
    let single_source = epoch_state.find_single_source_epochs();
    let redundant = epoch_state.find_redundant_epochs();
    
    let epoch_range = if !all_epochs.is_empty() {
        Some((*all_epochs.first().unwrap(), *all_epochs.last().unwrap()))
    } else {
        None
    };
    
    // Get storage information from the cached epoch state instead of triggering a scan
    let mut storages = Vec::new();
    for storage_config in &state.config.storage {
        let (id, storage_type, url, path) = match storage_config {
            crate::config::StorageConfig::Local(local) => {
                let id = format!("local:{}", local.path);
                (id.clone(), "local".to_string(), None, Some(local.path.clone()))
            }
            crate::config::StorageConfig::Http(http) => {
                let id = format!("http:{}", http.url);
                (id.clone(), "http".to_string(), Some(http.url.clone()), None)
            }
        };
        
        // Get epochs from the cached epoch state
        let epochs: Vec<u64> = epoch_state
            .get_storage_epochs(&id)
            .map(|epoch_set| {
                let mut epochs: Vec<u64> = epoch_set.iter().copied().collect();
                epochs.sort();
                epochs
            })
            .unwrap_or_default();
        
        storages.push(StorageStatus {
            id: id.clone(),
            storage_type,
            url,
            path,
            epoch_count: epochs.len(),
            epochs: epochs.into_iter().take(10).collect(), // Limit to first 10 for overview
        });
    }
    
    let elapsed = state.start_time.elapsed();
    
    Ok(Json(OverviewResponse {
        status: ApiStatus {
            version: env!("CARGO_PKG_VERSION").to_string(),
            uptime_seconds: elapsed.as_secs(),
            uptime_ms: elapsed.as_millis(),
            storage_count: state.config.storage.len(),
            epoch_count: all_epochs.len(),
        },
        storages,
        summary: EpochSummary {
            total_epochs: all_epochs.len(),
            complete_epochs,
            incomplete_epochs: all_epochs.len() - complete_epochs,
            single_source_epochs: single_source.len(),
            redundant_epochs: redundant.len(),
            epoch_range,
        },
    }))
}

pub async fn start_server(
    config: Arc<Config>,
    storage_manager: Arc<StorageManager>,
    epoch_state: Arc<RwLock<EpochState>>,
    port: u16,
) -> Result<()> {
    let state = AppState {
        config,
        storage_manager,
        epoch_state,
        start_time: Instant::now(),
    };
    
    let app = create_router(state);
    
    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{}", port))
        .await?;
    
    info!("HTTP API server listening on http://0.0.0.0:{}", port);
    
    axum::serve(listener, app).await?;
    
    Ok(())
}