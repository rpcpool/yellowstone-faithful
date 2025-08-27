use std::collections::{HashMap, HashSet};
use tracing::{debug, info};

use crate::storage::{CarFile, IndexFile, IndexType};

/// Represents the location of epoch data (CAR file)
#[derive(Debug, Clone)]
pub struct EpochLocation {
    pub storage_id: String,
    pub car_path: String,
    pub cid: Option<String>,
}

/// Represents the location of an index file
#[derive(Debug, Clone)]
pub struct IndexLocation {
    pub storage_id: String,
    pub index_type: IndexType,
    pub path: String,
}

/// Represents the complete state of all epochs across all storage backends
#[derive(Debug, Clone)]
pub struct EpochState {
    /// Map from epoch number to all storage locations that have the CAR file
    pub epochs: HashMap<u64, Vec<EpochLocation>>,
    
    /// Map from epoch number to all available indexes for that epoch
    pub epoch_indexes: HashMap<u64, Vec<IndexLocation>>,
    
    /// Reverse map: storage_id to set of epochs it contains (CAR files)
    pub storage_epochs: HashMap<String, HashSet<u64>>,
    
    /// Reverse map: storage_id to indexes it contains
    pub storage_indexes: HashMap<String, Vec<(u64, IndexType)>>,
    
    /// Track the total number of unique epochs
    pub total_epochs: usize,
}

impl EpochState {
    /// Create a new empty epoch state
    pub fn new() -> Self {
        Self {
            epochs: HashMap::new(),
            epoch_indexes: HashMap::new(),
            storage_epochs: HashMap::new(),
            storage_indexes: HashMap::new(),
            total_epochs: 0,
        }
    }
    
    /// Build epoch state from storage scan results
    pub fn from_scan_results(
        car_scan_results: &HashMap<String, Vec<CarFile>>,
        index_scan_results: &HashMap<String, Vec<IndexFile>>,
    ) -> Self {
        let mut state = Self::new();
        
        // Process CAR files
        for (storage_id, car_files) in car_scan_results {
            for car_file in car_files {
                state.add_epoch_location(
                    car_file.epoch,
                    storage_id.clone(),
                    car_file.path.clone(),
                    car_file.cid.clone(),
                );
            }
        }
        
        // Process index files
        for (storage_id, index_files) in index_scan_results {
            for index_file in index_files {
                state.add_index_location(
                    index_file.epoch,
                    storage_id.clone(),
                    index_file.index_type.clone(),
                    index_file.path.clone(),
                );
            }
        }
        
        state.total_epochs = state.epochs.len();
        info!("Built epoch state: {} unique epochs across {} storage locations", 
              state.total_epochs, state.storage_epochs.len());
        info!("Found indexes for {} epochs", state.epoch_indexes.len());
        
        state
    }
    
    /// Add a CAR file location for an epoch
    pub fn add_epoch_location(&mut self, epoch: u64, storage_id: String, car_path: String, cid: Option<String>) {
        let location = EpochLocation {
            storage_id: storage_id.clone(),
            car_path,
            cid,
        };
        
        // Add to epoch map
        self.epochs.entry(epoch)
            .or_insert_with(Vec::new)
            .push(location);
        
        // Add to storage map
        self.storage_epochs.entry(storage_id)
            .or_insert_with(HashSet::new)
            .insert(epoch);
            
        debug!("Added epoch {} CAR location", epoch);
    }
    
    /// Add an index location for an epoch
    pub fn add_index_location(&mut self, epoch: u64, storage_id: String, index_type: IndexType, path: String) {
        let location = IndexLocation {
            storage_id: storage_id.clone(),
            index_type: index_type.clone(),
            path,
        };
        
        // Add to epoch indexes map
        self.epoch_indexes.entry(epoch)
            .or_insert_with(Vec::new)
            .push(location);
        
        // Add to storage indexes map
        self.storage_indexes.entry(storage_id)
            .or_insert_with(Vec::new)
            .push((epoch, index_type.clone()));
            
        debug!("Added epoch {} index {:?} location", epoch, index_type);
    }
    
    /// Get all storage locations for a specific epoch
    pub fn get_epoch_locations(&self, epoch: u64) -> Option<&Vec<EpochLocation>> {
        self.epochs.get(&epoch)
    }
    
    /// Get the CID for an epoch (returns the first available CID)
    pub fn get_epoch_cid(&self, epoch: u64) -> Option<String> {
        self.epochs.get(&epoch).and_then(|locations| {
            locations.iter()
                .filter_map(|loc| loc.cid.clone())
                .next()
        })
    }
    
    /// Get all epochs available in a specific storage
    pub fn get_storage_epochs(&self, storage_id: &str) -> Option<&HashSet<u64>> {
        self.storage_epochs.get(storage_id)
    }
    
    /// Get a sorted list of all available epochs
    pub fn get_all_epochs(&self) -> Vec<u64> {
        let mut epochs: Vec<u64> = self.epochs.keys().copied().collect();
        epochs.sort();
        epochs
    }
    
    /// Find epochs that are only available in a single storage location
    pub fn find_single_source_epochs(&self) -> Vec<u64> {
        self.epochs.iter()
            .filter(|(_, locations)| locations.len() == 1)
            .map(|(epoch, _)| *epoch)
            .collect()
    }
    
    /// Find epochs that have redundancy (available in multiple locations)
    pub fn find_redundant_epochs(&self) -> Vec<u64> {
        self.epochs.iter()
            .filter(|(_, locations)| locations.len() > 1)
            .map(|(epoch, _)| *epoch)
            .collect()
    }
    
    /// Get storage utilization statistics
    pub fn get_storage_stats(&self) -> HashMap<String, StorageStats> {
        let mut stats = HashMap::new();
        
        for (storage_id, epochs) in &self.storage_epochs {
            stats.insert(storage_id.clone(), StorageStats {
                epoch_count: epochs.len(),
                epochs: epochs.iter().copied().collect(),
            });
        }
        
        stats
    }
    
    /// Check if an epoch has all required indexes
    pub fn is_epoch_complete(&self, epoch: u64) -> bool {
        if !self.epochs.contains_key(&epoch) {
            return false;
        }
        
        if let Some(indexes) = self.epoch_indexes.get(&epoch) {
            let index_types: HashSet<IndexType> = indexes.iter()
                .map(|loc| loc.index_type.clone())
                .collect();
            
            // Required indexes for Old Faithful
            let required = vec![
                IndexType::SlotToCid,
                IndexType::SigToCid,
                IndexType::CidToOffsetAndSize,
                IndexType::SigExists,
            ];
            
            required.iter().all(|req| index_types.contains(req))
        } else {
            false
        }
    }
    
    /// Get epochs that have CAR files but are missing indexes
    pub fn find_incomplete_epochs(&self) -> Vec<u64> {
        self.epochs.keys()
            .filter(|&&epoch| !self.is_epoch_complete(epoch))
            .copied()
            .collect()
    }
    
    /// Find the best storage location for an epoch (prefer local, then HTTP with most epochs)
    pub fn get_best_location(&self, epoch: u64) -> Option<&EpochLocation> {
        self.epochs.get(&epoch).and_then(|locations| {
            // Prefer local storage
            locations.iter()
                .find(|loc| loc.storage_id.starts_with("local:"))
                .or_else(|| {
                    // Otherwise, prefer storage with most epochs (likely more reliable)
                    locations.iter()
                        .max_by_key(|loc| {
                            self.storage_epochs.get(&loc.storage_id)
                                .map(|epochs| epochs.len())
                                .unwrap_or(0)
                        })
                })
        })
    }
    
    /// Generate a summary report of the epoch state
    pub fn summary(&self) -> String {
        let mut report = String::new();
        
        report.push_str(&format!("Total unique epochs: {}\n", self.total_epochs));
        report.push_str(&format!("Total storage locations: {}\n", self.storage_epochs.len()));
        
        let single_source = self.find_single_source_epochs();
        let redundant = self.find_redundant_epochs();
        
        report.push_str(&format!("Epochs with single source: {} ({:.1}%)\n", 
            single_source.len(),
            (single_source.len() as f64 / self.total_epochs as f64) * 100.0
        ));
        
        report.push_str(&format!("Epochs with redundancy: {} ({:.1}%)\n",
            redundant.len(),
            (redundant.len() as f64 / self.total_epochs as f64) * 100.0
        ));
        
        // Storage utilization
        report.push_str("\nStorage utilization:\n");
        for (storage_id, epochs) in &self.storage_epochs {
            report.push_str(&format!("  {}: {} epochs\n", storage_id, epochs.len()));
        }
        
        report
    }
}

/// Statistics for a storage location
#[derive(Debug, Clone)]
pub struct StorageStats {
    pub epoch_count: usize,
    pub epochs: Vec<u64>,
}

impl Default for EpochState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_epoch_state_building() {
        let mut scan_results = HashMap::new();
        
        // Add some test data
        scan_results.insert(
            "local:/test".to_string(),
            vec![
                CarFile { epoch: 0, path: "/test/epoch-0.car".to_string(), size: Some(1000), cid: Some("bafybeig123".to_string()) },
                CarFile { epoch: 1, path: "/test/epoch-1.car".to_string(), size: Some(2000), cid: None },
            ]
        );
        
        scan_results.insert(
            "http:example.com".to_string(),
            vec![
                CarFile { epoch: 1, path: "http://example.com/epoch-1.car".to_string(), size: None, cid: Some("bafybeig456".to_string()) },
                CarFile { epoch: 2, path: "http://example.com/epoch-2.car".to_string(), size: None, cid: None },
            ]
        );
        
        let index_results = HashMap::new();
        let state = EpochState::from_scan_results(&scan_results, &index_results);
        
        assert_eq!(state.total_epochs, 3);
        assert_eq!(state.get_all_epochs(), vec![0, 1, 2]);
        
        // Check epoch 1 has two locations
        let epoch1_locations = state.get_epoch_locations(1).unwrap();
        assert_eq!(epoch1_locations.len(), 2);
        
        // Check single source epochs
        let single_source = state.find_single_source_epochs();
        assert!(single_source.contains(&0));
        assert!(single_source.contains(&2));
        
        // Check redundant epochs
        let redundant = state.find_redundant_epochs();
        assert!(redundant.contains(&1));
    }
    
    #[test]
    fn test_best_location_selection() {
        let mut state = EpochState::new();
        
        // Add HTTP location first
        state.add_epoch_location(0, "http:example.com".to_string(), "http://example.com/epoch-0.car".to_string(), None);
        
        // Add local location second
        state.add_epoch_location(0, "local:/test".to_string(), "/test/epoch-0.car".to_string(), None);
        
        // Should prefer local even though it was added second
        let best = state.get_best_location(0).unwrap();
        assert!(best.storage_id.starts_with("local:"));
    }
}