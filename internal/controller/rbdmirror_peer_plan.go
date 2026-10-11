package controller

import (
	"fmt"
	"strings"

	"github.com/BenyaminSam/pook/internal/ceph"
)

type poolPeerActionKind string

const(
    poolPeerActionBootstrap = "bootstrap"
)

type poolPeerAction struct {
    kind            poolPeerActionKind
	poolName        string
	direction       string
	localMirrorUUID string
	peerMirrorUUID  string
}

type poolPeerConflict struct {
	poolName string
	message  string
}

type poolPeerPlan struct {
	actions   []poolPeerAction
	conflicts []poolPeerConflict
}

func (p poolPeerPlan) conflictMessage() string {
	messages := make([]string, 0, len(p.conflicts))

	for _, conflict := range p.conflicts {
		messages = append(messages, fmt.Sprintf("Pool %q: %s", conflict.poolName, conflict.message))
	}
	return strings.Join(messages, "; ")
}

func buildPoolPeerPlan(desired []desiredPoolMirroring, observations cephConnectionObservations) poolPeerPlan {
	var plan poolPeerPlan

	for _, pool := range desired {
		addConflict := func(message string) {
			plan.conflicts = append(plan.conflicts, poolPeerConflict{
				poolName: pool.name,
				message:  message,
			})
		}

		expected, err := expectedPoolPeerDirections(pool.direction)
		if err != nil {
			addConflict(err.Error())
			continue
		}

		local, localFound := observations.localPools[pool.name]
		peer, peerFound := observations.peerPools[pool.name]

		sides := []struct {
			role  mirroringClusterRole
			info  ceph.PoolMirroringInfo
			found bool
		}{
			{
				role:  mirroringClusterLocal,
				info:  local,
				found: localFound,
			},
			{
				role:  mirroringClusterPeer,
				info:  peer,
				found: peerFound,
			},
		}

		observationsValid := true

		for _, side := range sides {
			switch {
			case !side.found:
				addConflict(fmt.Sprintf("%s mirroring observation is missing", side.role))
				observationsValid = false

			case side.info.PoolName != pool.name:
				addConflict(fmt.Sprintf("%s observation identifies a different pool %q", side.role, side.info.PoolName))
				observationsValid = false

			case pool.cephPoolMode != "pool" || side.info.Mode != pool.cephPoolMode:
				addConflict(fmt.Sprintf("%s pool mode %q is not verified for desired mode %q", side.role, side.info.Mode, pool.cephPoolMode))
				observationsValid = false

			case side.info.MirrorUUID == "" || strings.TrimSpace(side.info.MirrorUUID) != side.info.MirrorUUID:
				addConflict(fmt.Sprintf("%s pool mirror UUID must be nonempty and have no surrounding whitespace", side.role))
				observationsValid = false
			}
		}

		if !observationsValid {
			continue
		}

		if local.MirrorUUID == peer.MirrorUUID {
			addConflict("Local and peer pools report the same mirror UUID, distinct pool identities are required")
			continue
		}

		localIdentity := classifyPoolPeerIdentity(local.Peers, peer.MirrorUUID)
		peerIdentity := classifyPoolPeerIdentity(peer.Peers, local.MirrorUUID)

		identityConflict := false

		if localIdentity.state == poolPeerIdentityConflict {
			addConflict(fmt.Sprintf("%s: %s", mirroringClusterLocal, localIdentity.message))
			identityConflict = true
		}

		if peerIdentity.state == poolPeerIdentityConflict {
			addConflict(fmt.Sprintf("%s: %s", mirroringClusterPeer, peerIdentity.message))
			identityConflict = true
		}

		if identityConflict {
			continue
		}

		switch {
		case localIdentity.state == poolPeerIdentityAbsent && peerIdentity.state == poolPeerIdentityAbsent:
			plan.actions = append(plan.actions, poolPeerAction{
                kind:            poolPeerActionBootstrap,
				poolName:        pool.name,
				direction:       pool.direction,
				localMirrorUUID: local.MirrorUUID,
				peerMirrorUUID:  peer.MirrorUUID,
			})

		case localIdentity.state == poolPeerIdentityMatched && peerIdentity.state == poolPeerIdentityMatched:
			if localIdentity.peer.Direction != expected.local {
				addConflict(fmt.Sprintf("%s registration %q has direction %q; expected %q", mirroringClusterLocal, localIdentity.peer.UUID, localIdentity.peer.Direction, expected.local))
			}
			if peerIdentity.peer.Direction != expected.peer {
				addConflict(fmt.Sprintf("%s registration %q has direction %q; expected %q", mirroringClusterPeer, peerIdentity.peer.UUID, peerIdentity.peer.Direction, expected.peer))
			}

		default:
			addConflict(fmt.Sprintf("peer relationship is incomplete; %s identity is %q and %s identity is %q; automatic repair is not implemented", mirroringClusterLocal, localIdentity.state, mirroringClusterPeer, peerIdentity.state))
		}
	}
	return plan
}
