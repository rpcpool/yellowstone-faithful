use anyhow::Result;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use tracing::{info, warn};

use crate::epoch_state::{EpochState, IndexLocation};
use crate::storage::IndexType;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FaithfulConfig {
    pub epoch: u64,
    pub version: u64,
    pub data: DataSection,
    pub indexes: IndexesSection,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DataSection {
    pub car: Option<CarSection>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CarSection {
    pub uri: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IndexesSection {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cid_to_offset_and_size: Option<IndexUri>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub slot_to_cid: Option<IndexUri>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sig_to_cid: Option<IndexUri>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sig_exists: Option<IndexUri>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gsfa: Option<IndexUri>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct IndexUri {
    pub uri: String,
}

pub struct ConfigGenerator {
    output_dir: PathBuf,
}

impl ConfigGenerator {
    pub fn new(output_dir: impl Into<PathBuf>) -> Self {
        Self {
            output_dir: output_dir.into(),
        }
    }
    
    /// Generate Old Faithful configuration files for all complete epochs
    pub async fn generate_configs(&self, epoch_state: &EpochState) -> Result<Vec<PathBuf>> {
        // Create output directory if it doesn't exist
        fs::create_dir_all(&self.output_dir)?;
        
        let mut generated_configs = Vec::new();
        let all_epochs = epoch_state.get_all_epochs();
        
        info!("Generating Old Faithful configuration files for {} epochs", all_epochs.len());
        
        for epoch in all_epochs {
            if !epoch_state.is_epoch_complete(epoch) {
                warn!("Skipping epoch {} - missing required indexes", epoch);
                continue;
            }
            
            if let Some(config) = self.generate_epoch_config(epoch, epoch_state)? {
                let filename = format!("epoch-{}.yaml", epoch);
                let output_path = self.output_dir.join(&filename);
                
                // Serialize to YAML
                let yaml_content = serde_yaml::to_string(&config)?;
                fs::write(&output_path, yaml_content)?;
                
                info!("Generated configuration for epoch {} -> {}", epoch, output_path.display());
                generated_configs.push(output_path);
            }
        }
        
        info!("Successfully generated {} configuration files", generated_configs.len());
        
        Ok(generated_configs)
    }
    
    /// Generate configuration for a single epoch
    fn generate_epoch_config(&self, epoch: u64, epoch_state: &EpochState) -> Result<Option<FaithfulConfig>> {
        // Get the best location for the CAR file
        let best_location = match epoch_state.get_best_location(epoch) {
            Some(loc) => loc,
            None => {
                warn!("No CAR file location found for epoch {}", epoch);
                return Ok(None);
            }
        };
        
        // Convert storage path to URI format
        let car_uri = self.storage_path_to_uri(&best_location.storage_id, &best_location.car_path);
        
        // Get index locations
        let indexes = match epoch_state.epoch_indexes.get(&epoch) {
            Some(idx) => idx,
            None => {
                warn!("No indexes found for epoch {}", epoch);
                return Ok(None);
            }
        };
        
        // Build indexes section
        let indexes_section = self.build_indexes_section(indexes);
        
        let config = FaithfulConfig {
            epoch,
            version: 1, // Old Faithful config version
            data: DataSection {
                car: Some(CarSection {
                    uri: car_uri,
                }),
            },
            indexes: indexes_section,
        };
        
        Ok(Some(config))
    }
    
    /// Convert storage-specific paths to URIs that Old Faithful can understand
    fn storage_path_to_uri(&self, storage_id: &str, path: &str) -> String {
        if storage_id.starts_with("local:") {
            // For local storage, use file:// URI
            if path.starts_with('/') {
                format!("file://{}", path)
            } else {
                format!("file://{}", path)
            }
        } else if storage_id.starts_with("http:") {
            // For HTTP storage, the path is already a URL
            path.to_string()
        } else {
            // Default: assume it's a path
            path.to_string()
        }
    }
    
    /// Build the indexes section from available index locations
    fn build_indexes_section(&self, indexes: &[IndexLocation]) -> IndexesSection {
        let mut section = IndexesSection {
            cid_to_offset_and_size: None,
            slot_to_cid: None,
            sig_to_cid: None,
            sig_exists: None,
            gsfa: None,
        };
        
        // Group indexes by type and prefer local storage
        let mut index_map: HashMap<IndexType, Vec<&IndexLocation>> = HashMap::new();
        for index in indexes {
            index_map.entry(index.index_type.clone())
                .or_insert_with(Vec::new)
                .push(index);
        }
        
        // For each index type, select the best location (prefer local)
        for (index_type, locations) in index_map {
            let best_location = locations.iter()
                .find(|loc| loc.storage_id.starts_with("local:"))
                .or_else(|| locations.first())
                .unwrap();
            
            let uri = self.storage_path_to_uri(&best_location.storage_id, &best_location.path);
            let index_uri = IndexUri { uri };
            
            match index_type {
                IndexType::CidToOffsetAndSize => section.cid_to_offset_and_size = Some(index_uri),
                IndexType::SlotToCid => section.slot_to_cid = Some(index_uri),
                IndexType::SigToCid => section.sig_to_cid = Some(index_uri),
                IndexType::SigExists => section.sig_exists = Some(index_uri),
                IndexType::Gsfa => section.gsfa = Some(index_uri),
            }
        }
        
        section
    }
    
    /// Generate a single combined configuration file for all epochs
    pub async fn generate_combined_config(&self, epoch_state: &EpochState) -> Result<PathBuf> {
        let all_epochs = epoch_state.get_all_epochs();
        let mut configs = Vec::new();
        
        for epoch in &all_epochs {
            if epoch_state.is_epoch_complete(*epoch) {
                if let Some(config) = self.generate_epoch_config(*epoch, epoch_state)? {
                    configs.push(config);
                }
            }
        }
        
        // Create a wrapper structure for multiple epochs
        #[derive(Serialize)]
        struct MultiEpochConfig {
            epochs: Vec<FaithfulConfig>,
        }
        
        let multi_config = MultiEpochConfig { epochs: configs };
        
        let output_path = self.output_dir.join("all-epochs.yaml");
        let yaml_content = serde_yaml::to_string(&multi_config)?;
        fs::write(&output_path, yaml_content)?;
        
        info!("Generated combined configuration file: {}", output_path.display());
        
        Ok(output_path)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::epoch_state::EpochLocation;
    use tempfile::TempDir;
    
    #[test]
    fn test_storage_path_to_uri() {
        let temp_dir = TempDir::new().unwrap();
        let generator = ConfigGenerator::new(temp_dir.path());
        
        // Test local path
        assert_eq!(
            generator.storage_path_to_uri("local:/storage", "/var/lib/faithful/epoch-0.car"),
            "file:///var/lib/faithful/epoch-0.car"
        );
        
        // Test HTTP URL
        assert_eq!(
            generator.storage_path_to_uri("http:example.com", "https://example.com/epoch-0.car"),
            "https://example.com/epoch-0.car"
        );
    }
    
    #[test]
    fn test_config_generation() {
        let mut epoch_state = EpochState::new();
        
        // Add a complete epoch
        epoch_state.add_epoch_location(
            500,
            "local:/storage".to_string(),
            "/var/lib/faithful/epoch-500.car".to_string(),
            Some("bafybeig123test".to_string()),
        );
        
        // Add required indexes
        epoch_state.add_index_location(
            500,
            "local:/storage".to_string(),
            IndexType::SlotToCid,
            "/var/lib/faithful/epoch-500-slot-to-cid.index".to_string(),
        );
        epoch_state.add_index_location(
            500,
            "local:/storage".to_string(),
            IndexType::SigToCid,
            "/var/lib/faithful/epoch-500-sig-to-cid.index".to_string(),
        );
        epoch_state.add_index_location(
            500,
            "local:/storage".to_string(),
            IndexType::CidToOffsetAndSize,
            "/var/lib/faithful/epoch-500-cid-to-offset-and-size.index".to_string(),
        );
        epoch_state.add_index_location(
            500,
            "local:/storage".to_string(),
            IndexType::SigExists,
            "/var/lib/faithful/epoch-500-sig-exists.index".to_string(),
        );
        
        let temp_dir = TempDir::new().unwrap();
        let generator = ConfigGenerator::new(temp_dir.path());
        
        let config = generator.generate_epoch_config(500, &epoch_state).unwrap().unwrap();
        
        assert_eq!(config.epoch, 500);
        assert_eq!(config.version, 1);
        assert!(config.data.car.is_some());
        assert!(config.indexes.slot_to_cid.is_some());
        assert!(config.indexes.sig_to_cid.is_some());
        assert!(config.indexes.cid_to_offset_and_size.is_some());
        assert!(config.indexes.sig_exists.is_some());
    }
}