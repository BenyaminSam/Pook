package controller

import (
	"fmt"
	"strings"

	"github.com/BenyaminSam/pook/internal/ceph"
)

type mirroringClusterRole string

const (
	mirroringClusterLocal mirroringClusterRole = "localCluster"
	mirroringClusterPeer  mirroringClusterRole = "peerCluster"
)

type poolMirroringAction struct {
	cluster     mirroringClusterRole
	poolName    string
	desiredMode string
}

type poolMirroringConflict struct {
	cluster  mirroringClusterRole
	poolName string
	message  string
}

type poolMirroringPlan struct {
	actions   []poolMirroringAction
	conflicts []poolMirroringConflict
}

func (p poolMirroringPlan) conflictMessage() string {
	messages := make([]string, 0, len(p.conflicts))

	for _, conflict := range p.conflicts {
		messages = append(messages, fmt.Sprintf("%s pool %q: %s", conflict.cluster, conflict.poolName, conflict.message))
	}
	return strings.Join(messages, "; ")
}

func buildPoolMirroringPlan(desired []desiredPoolMirroring, observations cephConnectionObservations) poolMirroringPlan {
	var plan poolMirroringPlan

	clusters := []struct {
		role  mirroringClusterRole
		pools map[string]ceph.PoolMirroringInfo
	}{
		{
			role:  mirroringClusterLocal,
			pools: observations.localPools,
		},
		{
			role:  mirroringClusterPeer,
			pools: observations.peerPools,
		},
	}

	for _, pool := range desired {
		for _, cluster := range clusters {
			info, found := cluster.pools[pool.name]
			if !found {
				plan.conflicts = append(plan.conflicts, poolMirroringConflict{
					cluster:  cluster.role,
					poolName: pool.name,
					message:  "Required mirroring observation is missing",
				})
				continue
			}

			if info.PoolName != pool.name {
				plan.conflicts = append(plan.conflicts, poolMirroringConflict{
					cluster:  cluster.role,
					poolName: pool.name,
					message:  fmt.Sprintf("observation identifies a different pool %q", info.PoolName),
				})
				continue
			}

			switch info.Mode {
			case pool.cephPoolMode:

			case "disabled":
				plan.actions = append(plan.actions, poolMirroringAction{
					cluster:     cluster.role,
					poolName:    pool.name,
					desiredMode: pool.cephPoolMode,
				})
			default:
				plan.conflicts = append(plan.conflicts, poolMirroringConflict{
					cluster:  cluster.role,
					poolName: pool.name,
					message:  fmt.Sprintf("Observed mode %q does not satisfy desired mode %q: automatic conversion is unsupported", info.Mode, pool.cephPoolMode),
				})
			}
		}
	}
	return plan
}
