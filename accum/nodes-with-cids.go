package accum

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/rpcpool/yellowstone-faithful/carreader"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/rpcpool/yellowstone-faithful/readasonecar"
	solanatxmetaparsers "github.com/rpcpool/yellowstone-faithful/solana-tx-meta-parsers"
	"github.com/rpcpool/yellowstone-faithful/tooling"
	"golang.org/x/sync/errgroup"
)

// DEPRECATED: Use nodetools instead.
func NewReader(reader readasonecar.CarReader) *Reader {
	return &Reader{
		reader: reader,
	}
}

// DEPRECATED: Use nodetools instead.
type Reader struct {
	reader readasonecar.CarReader
}

// DEPRECATED: Use nodetools instead.
// ReadUntilBlock reads the CAR file until it finds a block.
func (r *Reader) ReadUntilBlock(
	skipKinds iplddecoders.KindSlice,
) (*NodesWithCids, error) {
	nodesWithCids := GetNodesWithCids()
	for {
		offset, ok := r.reader.GetGlobalOffsetForNextRead()
		if !ok {
			break
		}
		_cid, sectionLength, buf, err := r.reader.NextNodeBytes()
		if err != nil {
			if err == io.EOF {
				break // End of file reached
			}
			return nil, fmt.Errorf("failed to read next node: %w", err)
		}
		data := buf.Bytes()
		kind, err := iplddecoders.GetKind(data)
		if err != nil {
			return nil, fmt.Errorf("failed to get kind for CID %s: %w", _cid.String(), err)
		}
		if skipKinds.Has(kind) {
			continue
		}
		decoded, err := iplddecoders.DecodeAny(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode node with CID %s: %w", _cid.String(), err)
		}
		nodeWithCid := &NodeWithCid{
			Cid:           _cid,
			Offset:        offset,
			SectionLength: sectionLength,
			Node:          decoded,
		}
		nodesWithCids.Push(nodeWithCid)
		carreader.PutBuffer(buf) // Return the buffer to the pool
		if nodeWithCid.IsBlock() {
			// If we encounter a block, we stop reading further.
			break
		}
	}
	return nodesWithCids, nil
}

func (r *Reader) Put(nodes *NodesWithCids) {
	for _, decoded := range nodes.nodes {
		iplddecoders.PutAny(decoded.Node)
	}
	PutNodesWithCids(nodes)
}

type NodeWithCid struct {
	Cid           cid.Cid
	Offset        uint64
	SectionLength uint64
	Node          ipldbindcode.Node
}

// IsDataFrame checks if the NodeWithCid is a DataFrame.
func (n *NodeWithCid) IsDataFrame() bool {
	_, ok := n.Node.(*ipldbindcode.DataFrame)
	return ok
}

func (n *NodeWithCid) IsBlock() bool {
	_, ok := n.Node.(*ipldbindcode.Block)
	return ok
}

func (n *NodeWithCid) DataFrame() (*ipldbindcode.DataFrame, bool) {
	dataFrame, ok := n.Node.(*ipldbindcode.DataFrame)
	if !ok {
		return nil, false
	}
	return dataFrame, true
}

// pub struct NodesWithCids(pub Vec<NodeWithCid>);
type NodesWithCids struct {
	nodes []*NodeWithCid
}

func NewNodesWithCids() NodesWithCids {
	return NodesWithCids{}
}

var nodesWithCidsPool = &sync.Pool{
	New: func() interface{} {
		return &NodesWithCids{}
	},
}

func GetNodesWithCids() *NodesWithCids {
	return nodesWithCidsPool.Get().(*NodesWithCids)
}

func PutNodesWithCids(n *NodesWithCids) {
	// Clear the slice to avoid memory leaks
	n.nodes = n.nodes[:0]
	nodesWithCidsPool.Put(n)
}

// impl NodesWithCids {
//     pub fn new() -> NodesWithCids {
//         NodesWithCids(vec![])
//     }

//	pub fn push(&mut self, node_with_cid: NodeWithCid) {
//	    self.0.push(node_with_cid);
//	}
func (n *NodesWithCids) Push(nodeWithCid *NodeWithCid) {
	n.nodes = append(n.nodes, nodeWithCid)
}

//	pub fn len(&self) -> usize {
//	    self.0.len()
//	}
func (n *NodesWithCids) Len() int {
	return len(n.nodes)
}

//	pub fn is_empty(&self) -> bool {
//	    self.len() == 0
//	}
func (n *NodesWithCids) IsEmpty() bool {
	return n.Len() == 0
}

//	pub fn get(&self, index: usize) -> &NodeWithCid {
//	    &self.0[index]
//	}
func (n *NodesWithCids) Get(index int) *NodeWithCid {
	if index < 0 || index >= len(n.nodes) {
		return nil // or handle the error as needed
	}
	return n.nodes[index]
}

//	pub fn get_by_cid(&self, cid: &Cid) -> Option<&NodeWithCid> {
//	    self.0
//	        .iter()
//	        .find(|&node_with_cid| node_with_cid.get_cid() == cid)
//	}
func (n *NodesWithCids) GetByCid(cid *cid.Cid) *NodeWithCid {
	for _, nodeWithCid := range n.nodes {
		if nodeWithCid.Cid.Equals(*cid) {
			return nodeWithCid
		}
	}
	return nil // or handle the error as needed
}

//     pub fn reassemble_dataframes(
//         &self,
//         first_dataframe: dataframe::DataFrame,
//     ) -> Result<Vec<u8>, Box<dyn Error>> {
//         let mut data = first_dataframe.data.to_vec();
//         let mut next_arr = first_dataframe.next;
//         while next_arr.is_some() {
//             for next_cid in next_arr.clone().unwrap() {
//                 let next_node = self.get_by_cid(&next_cid);
//                 if next_node.is_none() {
//                     return Err(Box::new(std::io::Error::new(
//                         std::io::ErrorKind::Other,
//                         std::format!("Missing CID: {:?}", next_cid),
//                     )));
//                 }
//                 let next_node_un = next_node.unwrap();

//                 if !next_node_un.get_node().is_dataframe() {
//                     return Err(Box::new(std::io::Error::new(
//                         std::io::ErrorKind::Other,
//                         std::format!("Expected DataFrame, got {:?}", next_node_un.get_node()),
//                     )));
//                 }

//                 let next_dataframe = next_node_un.get_node().get_dataframe().unwrap();
//                 data.extend(next_dataframe.data.to_vec());
//                 next_arr.clone_from(&next_dataframe.next);
//             }
//         }

//	    if first_dataframe.hash.is_some() {
//	        let wanted_hash = first_dataframe.hash.unwrap();
//	        verify_hash(data.clone(), wanted_hash)?;
//	    }
//	    Ok(data)
//	}
func (n *NodesWithCids) ReassembleDataframes(firstDataFrame *ipldbindcode.DataFrame) ([]byte, error) {
	return ipldbindcode.LoadDataFromDataFrames(firstDataFrame, func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error) {
		nodeWithCid := n.GetByCid(&wantedCid)
		if nodeWithCid == nil {
			return nil, fmt.Errorf("missing CID: %s", wantedCid.String())
		}
		dataFrame, ok := nodeWithCid.Node.(*ipldbindcode.DataFrame)
		if !ok {
			return nil, fmt.Errorf("expected DataFrame, got %T", nodeWithCid.Node)
		}
		return dataFrame, nil
	})
}

// pub fn each<F>(&self, mut f: F) -> Result<(), Box<dyn Error>>
// where
//
//	F: FnMut(&NodeWithCid) -> Result<(), Box<dyn Error>>,
//
//	{
//	    for node_with_cid in &self.0 {
//	        f(node_with_cid)?;
//	    }
//	    Ok(())
//	}
func (n *NodesWithCids) Each(f func(*NodeWithCid) error) error {
	wg := new(errgroup.Group)
	for _, nodeWithCid := range n.nodes {
		wg.Go(func() error {
			return f(nodeWithCid)
		})
	}
	return wg.Wait()
}

//	pub fn get_cids(&self) -> Vec<Cid> {
//	    let mut cids = vec![];
//	    for node_with_cid in &self.0 {
//	        cids.push(*node_with_cid.get_cid());
//	    }
//	    cids
//	}
func (n *NodesWithCids) GetCids() []cid.Cid {
	cids := make([]cid.Cid, 0, len(n.nodes))
	for _, nodeWithCid := range n.nodes {
		cids = append(cids, nodeWithCid.Cid)
	}
	return cids
}

//	pub fn get_block(&self) -> Result<&block::Block, Box<dyn Error>> {
//	    // the last node should be a block
//	    let last_node = self.0.last();
//	    if last_node.is_none() {
//	        return Err(Box::new(std::io::Error::new(
//	            std::io::ErrorKind::Other,
//	            "No nodes".to_owned(),
//	        )));
//	    }
//	    let last_node_un = last_node.unwrap();
//	    if !last_node_un.get_node().is_block() {
//	        return Err(Box::new(std::io::Error::new(
//	            std::io::ErrorKind::Other,
//	            std::format!("Expected Block, got {:?}", last_node_un.get_node()),
//	        )));
//	    }
//	    let block = last_node_un.get_node().get_block().unwrap();
//	    Ok(block)
//	}
func (n *NodesWithCids) GetBlock() (*ipldbindcode.Block, error) {
	// the last node should be a block
	if len(n.nodes) == 0 {
		return nil, fmt.Errorf("no nodes")
	}
	lastNode := n.nodes[len(n.nodes)-1]
	block, ok := lastNode.Node.(*ipldbindcode.Block)
	if !ok {
		return nil, fmt.Errorf("expected Block, got %T", lastNode.Node)
	}
	return block, nil
}

// }
func (n *NodesWithCids) GetTransactions() ([]*TransactionWithSlot, error) {
	transactions := getTransactionWithSlotSlice()
	block, err := n.GetBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get block from nodes: %w", err)
	}
	mu := &sync.Mutex{}
	err = n.Each(func(nodeWithCid *NodeWithCid) error {
		switch node := nodeWithCid.Node.(type) {
		case *ipldbindcode.Transaction:
			tx, err := node.GetSolanaTransaction()
			if err != nil {
				return fmt.Errorf("error while getting solana transaction from object %s: %w", nodeWithCid.Cid, err)
			}
			tws := &TransactionWithSlot{
				Offset:    nodeWithCid.Offset,
				Length:    nodeWithCid.SectionLength,
				Slot:      uint64(node.Slot),
				Blocktime: uint64(block.Meta.Blocktime),
			}
			tws.Transaction = tx
			sig := tx.Signatures[0]

			metaBuffer, err := n.ReassembleDataframes(&node.Metadata)
			if err != nil {
				return fmt.Errorf("failed to reassemble dataframes for transaction %s: %w", sig, err)
			}
			if len(metaBuffer) > 0 {
				uncompressedMeta, err := tooling.DecompressZstd(metaBuffer)
				if err != nil {
					return fmt.Errorf("failed to decompress metadata for transaction %s: %w", sig, err)
				}
				status, err := solanatxmetaparsers.ParseTransactionStatusMetaContainer(uncompressedMeta)
				if err == nil {
					tws.Metadata = status
				} else {
					tws.Error = newTxMetaErrorParseError(sig, err)
				}
			} else {
				tws.Error = newTxMetaErrorNotFound(sig, fmt.Errorf("metadata is empty"))
			}
			mu.Lock()
			defer mu.Unlock()
			transactions = append(transactions, tws)
		default:
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error while iterating nodes: %w", err)
	}
	return transactions, nil
}
