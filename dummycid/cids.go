package dummycid

import "github.com/ipfs/go-cid"

// DummyCID is the "zero-length "identity" multihash with "raw" codec".
//
// This is the best-practices placeholder value to refer to a non-existent or unknown object.
var DummyCID = cid.MustParse("bafkqaaa")
