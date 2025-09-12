mod cid_to_offset;
mod sig_to_cid;
mod slot_to_cid;

pub use cid_to_offset::CidToOffsetAndSize;
pub use sig_to_cid::SigToCid;
pub use slot_to_cid::SlotToCid;

/// Common metadata keys used by indexes
pub mod metadata_keys {
    pub const KIND: &[u8] = b"index_kind";
    pub const EPOCH: &[u8] = b"epoch";
    pub const ROOT_CID: &[u8] = b"root_cid";
    pub const NETWORK: &[u8] = b"network";
}