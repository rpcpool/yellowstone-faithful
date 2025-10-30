pub mod block;
pub mod transaction;

pub use block::*;
pub use transaction::*;

use serde::{Deserialize, Serialize};

/// Version information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VersionInfo {
    pub version: String,
}

/// Block time information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BlockTime {
    pub slot: u64,
    pub block_time: i64,
}

/// Streaming filter for blocks
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct StreamBlocksFilter {
    /// Filter blocks/transactions mentioning these accounts
    pub account_include: Vec<String>,
}

/// Streaming filter for transactions
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct StreamTransactionsFilter {
    /// Include/exclude vote transactions
    pub vote: Option<bool>,

    /// Include/exclude failed transactions
    pub failed: Option<bool>,

    /// Filter transactions mentioning these accounts
    pub account_include: Vec<String>,

    /// Exclude transactions mentioning these accounts
    pub account_exclude: Vec<String>,

    /// Require transactions to mention all of these accounts
    pub account_required: Vec<String>,
}
