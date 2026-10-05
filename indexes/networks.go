package indexes

import (
	"github.com/rpcpool/yellowstone-faithful/slottools"
	"k8s.io/klog/v2"
)

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

// EpochSchedule returns the epoch schedule of the network.
func (n Network) EpochSchedule() (slottools.EpochSchedule, error) {
	return slottools.EpochScheduleForNetwork(string(n))
}

// ApplyEpochSchedule makes the network's epoch schedule the process-wide one
// (see slottools.SetEpochSchedule) and logs it.
func (n Network) ApplyEpochSchedule() error {
	s, err := n.EpochSchedule()
	if err != nil {
		return err
	}
	slottools.SetEpochSchedule(s)
	klog.Infof("Using %s epoch schedule: %s", n, s)
	return nil
}
