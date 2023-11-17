package main

import (
	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/radiance/genesis"
)

type GenesisContainer struct {
	Hash solana.Hash
	// The genesis config.
	Config *genesis.Genesis
}
