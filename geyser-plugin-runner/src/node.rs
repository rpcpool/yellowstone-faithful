use bytes::Buf;
use cid::Cid;
use core::hash::Hasher;
use crc::{Crc, CRC_64_GO_ISO};
use fnv::FnvHasher;
use multihash;

use serde_cbor;

use std::error::Error;
use std::fs::File;
use std::io::{self, BufReader, Read};
use std::vec::Vec;

use crate::block;
use crate::dataframe;
use crate::entry;
use crate::epoch;
use crate::rewards;
use crate::subset;
use crate::transaction;
use crate::utils;

pub struct NodeWithCid {
    cid: Cid,
    node: Node,
}

impl NodeWithCid {
    pub fn new(cid: Cid, node: Node) -> NodeWithCid {
        NodeWithCid { cid, node }
    }

    pub fn get_cid(&self) -> &Cid {
        &self.cid
    }

    pub fn get_node(&self) -> &Node {
        &self.node
    }
}

pub struct NodesWithCids(pub Vec<NodeWithCid>);

impl NodesWithCids {
    pub fn new() -> NodesWithCids {
        NodesWithCids(vec![])
    }

    pub fn push(&mut self, node_with_cid: NodeWithCid) {
        self.0.push(node_with_cid);
    }

    pub fn len(&self) -> usize {
        self.0.len()
    }

    pub fn get(&self, index: usize) -> &NodeWithCid {
        &self.0[index]
    }

    pub fn get_by_cid(&self, cid: &Cid) -> Option<&NodeWithCid> {
        for node_with_cid in &self.0 {
            if node_with_cid.get_cid() == cid {
                return Some(node_with_cid);
            }
        }
        return None;
    }

    pub fn reassemble_dataframes(
        &self,
        first_dataframe: dataframe::DataFrame,
    ) -> Result<Vec<u8>, Box<dyn Error>> {
        let mut data = first_dataframe.data.to_vec();
        let mut next_arr = first_dataframe.next;
        while next_arr.is_some() {
            for next_cid in next_arr.clone().unwrap() {
                let next_node = self.get_by_cid(&next_cid);
                if next_node.is_none() {
                    return Err(Box::new(std::io::Error::new(
                        std::io::ErrorKind::Other,
                        std::format!("Missing CID: {:?}", next_cid),
                    )));
                }
                let next_node_un = next_node.unwrap();

                if !next_node_un.get_node().is_dataframe() {
                    return Err(Box::new(std::io::Error::new(
                        std::io::ErrorKind::Other,
                        std::format!("Expected DataFrame, got {:?}", next_node_un.get_node()),
                    )));
                }

                let next_dataframe = next_node_un.get_node().get_dataframe().unwrap();
                data.extend(next_dataframe.data.to_vec());
                next_arr = next_dataframe.next.clone();
            }
        }

        if first_dataframe.hash.is_some() {
            let wanted_hash = first_dataframe.hash.unwrap();
            verify_hash(data.clone(), wanted_hash)?;
        }
        return Ok(data);
    }

    pub fn each<F>(&self, mut f: F) -> Result<(), Box<dyn Error>>
    where
        F: FnMut(&NodeWithCid) -> Result<(), Box<dyn Error>>,
    {
        for node_with_cid in &self.0 {
            f(node_with_cid)?;
        }
        return Ok(());
    }

    pub fn get_cids(&self) -> Vec<Cid> {
        let mut cids = vec![];
        for node_with_cid in &self.0 {
            cids.push(node_with_cid.get_cid().clone());
        }
        return cids;
    }

    pub fn get_block(&self) -> Result<&block::Block, Box<dyn Error>> {
        // the last node should be a block
        let last_node = self.0.last();
        if last_node.is_none() {
            return Err(Box::new(std::io::Error::new(
                std::io::ErrorKind::Other,
                std::format!("No nodes"),
            )));
        }
        let last_node_un = last_node.unwrap();
        if !last_node_un.get_node().is_block() {
            return Err(Box::new(std::io::Error::new(
                std::io::ErrorKind::Other,
                std::format!("Expected Block, got {:?}", last_node_un.get_node()),
            )));
        }
        let block = last_node_un.get_node().get_block().unwrap();
        return Ok(block);
    }
}

pub fn verify_hash(data: Vec<u8>, hash: u64) -> Result<(), Box<dyn Error>> {
    let crc64 = checksum_crc64(&data);
    if crc64 != hash {
        // Maybe it's the legacy checksum function?
        let fnv = checksum_fnv(&data);
        if fnv != hash {
            return Err(Box::new(std::io::Error::new(
                std::io::ErrorKind::Other,
                std::format!(
                    "data hash mismatch: wanted {:?}, got crc64={:?}, fnv={:?}",
                    hash,
                    crc64,
                    fnv
                ),
            )));
        }
    }
    return Ok(());
}

fn checksum_crc64(data: &Vec<u8>) -> u64 {
    let crc = Crc::<u64>::new(&CRC_64_GO_ISO);
    let mut digest = crc.digest();
    digest.update(data);
    let crc64 = digest.finalize();
    crc64
}

fn checksum_fnv(data: &Vec<u8>) -> u64 {
    let mut hasher = FnvHasher::default();
    hasher.write(data);
    let hash = hasher.finish();
    hash
}

#[derive(Clone, PartialEq, Eq, Hash, Debug)]
pub enum Node {
    Transaction(transaction::Transaction),
    Entry(entry::Entry),
    Block(block::Block),
    Subset(subset::Subset),
    Epoch(epoch::Epoch),
    Rewards(rewards::Rewards),
    DataFrame(dataframe::DataFrame),
}

impl Node {
    pub fn is_transaction(&self) -> bool {
        match self {
            Node::Transaction(_) => true,
            _ => false,
        }
    }
    pub fn is_entry(&self) -> bool {
        match self {
            Node::Entry(_) => true,
            _ => false,
        }
    }
    pub fn is_block(&self) -> bool {
        match self {
            Node::Block(_) => true,
            _ => false,
        }
    }
    pub fn is_subset(&self) -> bool {
        match self {
            Node::Subset(_) => true,
            _ => false,
        }
    }
    pub fn is_epoch(&self) -> bool {
        match self {
            Node::Epoch(_) => true,
            _ => false,
        }
    }
    pub fn is_rewards(&self) -> bool {
        match self {
            Node::Rewards(_) => true,
            _ => false,
        }
    }
    pub fn is_dataframe(&self) -> bool {
        match self {
            Node::DataFrame(_) => true,
            _ => false,
        }
    }

    pub fn get_transaction(&self) -> Option<&transaction::Transaction> {
        match self {
            Node::Transaction(transaction) => Some(transaction),
            _ => None,
        }
    }
    pub fn get_entry(&self) -> Option<&entry::Entry> {
        match self {
            Node::Entry(entry) => Some(entry),
            _ => None,
        }
    }
    pub fn get_block(&self) -> Option<&block::Block> {
        match self {
            Node::Block(block) => Some(block),
            _ => None,
        }
    }
    pub fn get_subset(&self) -> Option<&subset::Subset> {
        match self {
            Node::Subset(subset) => Some(subset),
            _ => None,
        }
    }
    pub fn get_epoch(&self) -> Option<&epoch::Epoch> {
        match self {
            Node::Epoch(epoch) => Some(epoch),
            _ => None,
        }
    }
    pub fn get_rewards(&self) -> Option<&rewards::Rewards> {
        match self {
            Node::Rewards(rewards) => Some(rewards),
            _ => None,
        }
    }
    pub fn get_dataframe(&self) -> Option<&dataframe::DataFrame> {
        match self {
            Node::DataFrame(dataframe) => Some(dataframe),
            _ => None,
        }
    }
}

// parse_any_from_cbordata parses any CBOR data into either a Epoch, Subset, Block, Rewards, Entry, or Transaction
pub fn parse_any_from_cbordata(data: Vec<u8>) -> Result<Node, Box<dyn Error>> {
    let decoded_data: serde_cbor::Value = serde_cbor::from_slice(&data).unwrap();
    // Process the decoded data
    // println!("Data: {:?}", decoded_data);
    let cloned_data = decoded_data.clone();

    // decoded_data is an serde_cbor.Array; print the kind, which is the first element of the array
    if let serde_cbor::Value::Array(array) = decoded_data {
        // println!("Kind: {:?}", array[0]);
        if let Some(serde_cbor::Value::Integer(kind)) = array.get(0) {
            // println!(
            //     "Kind: {:?}",
            //     Kind::from_u64(kind as u64).unwrap().to_string()
            // );

            // based on the kind, we can decode the rest of the data
            match kind {
                kind => match Kind::from_u64(kind as u64).unwrap() {
                    Kind::Transaction => {
                        let transaction = transaction::Transaction::from_cbor(cloned_data)?;
                        return Ok(Node::Transaction(transaction));
                    }
                    Kind::Entry => {
                        let entry = entry::Entry::from_cbor(cloned_data)?;
                        return Ok(Node::Entry(entry));
                    }
                    Kind::Block => {
                        let block = block::Block::from_cbor(cloned_data)?;
                        return Ok(Node::Block(block));
                    }
                    Kind::Subset => {
                        let subset = subset::Subset::from_cbor(cloned_data)?;
                        return Ok(Node::Subset(subset));
                    }
                    Kind::Epoch => {
                        let epoch = epoch::Epoch::from_cbor(cloned_data)?;
                        return Ok(Node::Epoch(epoch));
                    }
                    Kind::Rewards => {
                        let rewards = rewards::Rewards::from_cbor(cloned_data)?;
                        return Ok(Node::Rewards(rewards));
                    }
                    Kind::DataFrame => {
                        let dataframe = dataframe::DataFrame::from_cbor(cloned_data)?;
                        return Ok(Node::DataFrame(dataframe));
                    }
                    unknown => {
                        return Err(Box::new(std::io::Error::new(
                            std::io::ErrorKind::Other,
                            std::format!("Unknown type: {:?}", unknown),
                        )))
                    }
                },
                unknown => {
                    return Err(Box::new(std::io::Error::new(
                        std::io::ErrorKind::Other,
                        std::format!("Unknown type: {:?}", unknown),
                    )))
                }
            }
        }
    }

    return Err(Box::new(std::io::Error::new(
        std::io::ErrorKind::Other,
        std::format!("Unknown type"),
    )));
}

pub enum Kind {
    Transaction,
    Entry,
    Block,
    Subset,
    Epoch,
    Rewards,
    DataFrame,
}

impl std::fmt::Debug for Kind {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Kind")
            .field("kind", &self.to_string())
            .finish()
    }
}

impl Kind {
    pub fn from_u64(kind: u64) -> Option<Kind> {
        match kind {
            0 => Some(Kind::Transaction),
            1 => Some(Kind::Entry),
            2 => Some(Kind::Block),
            3 => Some(Kind::Subset),
            4 => Some(Kind::Epoch),
            5 => Some(Kind::Rewards),
            6 => Some(Kind::DataFrame),
            _ => None,
        }
    }

    pub fn to_u64(&self) -> u64 {
        match self {
            Kind::Transaction => 0,
            Kind::Entry => 1,
            Kind::Block => 2,
            Kind::Subset => 3,
            Kind::Epoch => 4,
            Kind::Rewards => 5,
            Kind::DataFrame => 6,
        }
    }

    pub fn to_string(&self) -> String {
        match self {
            Kind::Transaction => "Transaction".to_string(),
            Kind::Entry => "Entry".to_string(),
            Kind::Block => "Block".to_string(),
            Kind::Subset => "Subset".to_string(),
            Kind::Epoch => "Epoch".to_string(),
            Kind::Rewards => "Rewards".to_string(),
            Kind::DataFrame => "DataFrame".to_string(),
        }
    }
}

pub struct RawNode {
    cid: Cid,
    data: Vec<u8>,
}

// Debug trait for RawNode
impl std::fmt::Debug for RawNode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("RawNode")
            .field("cid", &self.cid)
            .field("data", &self.data)
            .finish()
    }
}

impl RawNode {
    pub fn new(cid: Cid, data: Vec<u8>) -> RawNode {
        RawNode { cid, data }
    }

    pub fn parse(&self) -> Result<Node, Box<dyn Error>> {
        let parsed = parse_any_from_cbordata(self.data.clone());
        if parsed.is_err() {
            println!("Error: {:?}", parsed.err().unwrap());
        } else {
            let node = parsed.unwrap();
            return Ok(node);
        }
        return Err(Box::new(std::io::Error::new(
            std::io::ErrorKind::Other,
            std::format!("Unknown type"),
        )));
    }

    pub fn from_cursor(cursor: &mut io::Cursor<Vec<u8>>) -> Result<RawNode, Box<dyn Error>> {
        let cid_version = utils::read_uvarint(cursor)?;
        // println!("CID version: {}", cid_version);

        let multicodec = utils::read_uvarint(cursor)?;
        // println!("Multicodec: {}", multicodec);

        // Multihash hash function code.
        let hash_function = utils::read_uvarint(cursor)?;
        // println!("Hash function: {}", hash_function);

        // Multihash digest length.
        let digest_length = utils::read_uvarint(cursor)?;
        // println!("Digest length: {}", digest_length);

        if digest_length > 64 {
            return Err(Box::new(std::io::Error::new(
                std::io::ErrorKind::Other,
                std::format!("Digest length too long"),
            )));
        }

        // reac actual digest
        let mut digest = vec![0u8; digest_length as usize];
        cursor.read_exact(&mut digest)?;

        // the rest is the data
        let mut data = vec![];
        cursor.read_to_end(&mut data)?;

        // println!("Data: {:?}", data);

        let ha = multihash::Multihash::wrap(hash_function, digest.as_slice())?;

        match cid_version {
            0 => {
                let cid = Cid::new_v0(ha)?;
                let raw_node = RawNode::new(cid, data);
                return Ok(raw_node);
            }
            1 => {
                let cid = Cid::new_v1(multicodec, ha);
                let raw_node = RawNode::new(cid, data);
                return Ok(raw_node);
            }
            _ => {
                return Err(Box::new(std::io::Error::new(
                    std::io::ErrorKind::Other,
                    std::format!("Unknown CID version"),
                )));
            }
        }
    }
}

pub struct NodeReader {
    reader: BufReader<File>,
    header: Vec<u8>,
    item_index: u64,
}

impl NodeReader {
    pub fn new(file_path: String) -> Result<NodeReader, Box<dyn Error>> {
        let file = File::open(file_path)?;
        // create a buffered reader over the file
        let MiB = 1024 * 1024;
        let capacity = MiB * 8;
        let reader = BufReader::with_capacity(capacity, file);

        let node_reader = NodeReader {
            reader,
            header: vec![],
            item_index: 0,
        };
        return Ok(node_reader);
    }

    pub fn read_raw_header(&mut self) -> Result<Vec<u8>, Box<dyn Error>> {
        if self.header.len() > 0 {
            return Ok(self.header.clone());
        };
        let header_length = utils::read_uvarint(&mut self.reader)?;
        if header_length > 1024 {
            return Err(Box::new(std::io::Error::new(
                std::io::ErrorKind::Other,
                std::format!("Header length too long"),
            )));
        }
        let mut header = vec![0u8; header_length as usize];
        self.reader.read_exact(&mut header)?;

        self.header = header.clone();

        let clone = header.clone();
        return Ok(clone.as_slice().to_owned());
    }

    pub fn next(&mut self) -> Result<RawNode, Box<dyn Error>> {
        if self.header.len() == 0 {
            self.read_raw_header()?;
        };

        // println!("Item index: {}", item_index);
        self.item_index += 1;

        // Read and decode the uvarint prefix (length of CID + data)
        let section_size = utils::read_uvarint(&mut self.reader)?;
        // println!("Section size: {}", section_size);

        if section_size > utils::MAX_ALLOWED_SECTION_SIZE as u64 {
            return Err(Box::new(std::io::Error::new(
                std::io::ErrorKind::Other,
                std::format!("Section size too long"),
            )));
        }

        // read whole item
        let mut item = vec![0u8; section_size as usize];
        self.reader.read_exact(&mut item)?;

        // dump item bytes as numbers
        // println!("Item bytes: {:?}", item);

        // now create a cursor over the item
        let mut cursor = io::Cursor::new(item);

        return RawNode::from_cursor(&mut cursor);
    }

    pub fn next_parsed(&mut self) -> Result<NodeWithCid, Box<dyn Error>> {
        let raw_node = self.next()?;
        let cid = raw_node.cid.clone();
        return Ok(NodeWithCid::new(cid, raw_node.parse()?));
    }

    pub fn read_until_block(&mut self) -> Result<NodesWithCids, Box<dyn Error>> {
        let mut nodes = NodesWithCids::new();
        loop {
            let node = self.next_parsed()?;
            if node.get_node().is_block() {
                nodes.push(node);
                break;
            }
            nodes.push(node);
        }
        return Ok(nodes);
    }

    pub fn get_item_index(&self) -> u64 {
        self.item_index
    }
}
