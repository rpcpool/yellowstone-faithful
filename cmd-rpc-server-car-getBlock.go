package main

import (
	"time"

	"k8s.io/klog/v2"
)

type timer struct {
	start time.Time
	prev  time.Time
}

func newTimer() *timer {
	now := time.Now()
	return &timer{
		start: now,
		prev:  now,
	}
}

func (t *timer) time(name string) {
	klog.V(2).Infof("TIMED: %s: %s (overall %s)", name, time.Since(t.prev), time.Since(t.start))
	t.prev = time.Now()
}

//	pub enum RewardType {
//	    Fee,
//	    Rent,
//	    Staking,
//	    Voting,
//	}
func rewardTypeToString(typ int) string {
	switch typ {
	case 1:
		return "Fee"
	case 2:
		return "Rent"
	case 3:
		return "Staking"
	case 4:
		return "Voting"
	default:
		return "Unknown"
	}
}

func rewardTypeStringToInt(typ string) int {
	switch typ {
	case "Fee":
		return 1
	case "Rent":
		return 2
	case "Staking":
		return 3
	case "Voting":
		return 4
	default:
		return 0
	}
}

const CodeNotFound = -32009
