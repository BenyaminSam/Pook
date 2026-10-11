package controller

import (
	"fmt"
)

type poolPeerDirections struct {
	local string
	peer  string
}

func expectedPoolPeerDirections(direction string) (poolPeerDirections, error) {
	switch direction {
	case "rx-only":
		return poolPeerDirections{
			local: "tx-only",
			peer:  "rx-only",
		}, nil
	case "rx-tx":
		return poolPeerDirections{
			local: "rx-tx",
			peer:  "rx-tx",
		}, nil
	default:
		return poolPeerDirections{}, fmt.Errorf("unsupported pool mirroring direction %q", direction)
	}
}
