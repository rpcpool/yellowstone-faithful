package indexes

type Network string

const (
	NetworkMainnet Network = "mainnet"
	NetworkTestnet Network = "testnet"
	NetworkDevnet  Network = "devnet"
)

func IsValidNetwork(network Network) bool {
	switch network {
	case NetworkMainnet, NetworkTestnet, NetworkDevnet:
		return true
	default:
		return false
	}
}
