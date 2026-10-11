package controller

import(
    "fmt"
    "strings"

    "github.com/BenyaminSam/pook/internal/ceph"
)

type poolPeerIdentityState string

const(
    poolPeerIdentityAbsent   poolPeerIdentityState = "absent"
	poolPeerIdentityMatched  poolPeerIdentityState = "matched"
	poolPeerIdentityConflict poolPeerIdentityState = "conflict"
)

type poolPeerIdentityResult struct {
    state       poolPeerIdentityState
    peer        ceph.PoolMirroringPeer
    message     string
}

func classifyPoolPeerIdentity(peers []ceph.PoolMirroringPeer, remoteMirrorUUID string) poolPeerIdentityResult {
    conflict := func(message string) poolPeerIdentityResult{
        return poolPeerIdentityResult{
            state:      poolPeerIdentityConflict,
            message:    message,
        }
    }

    if remoteMirrorUUID == "" || strings.TrimSpace(remoteMirrorUUID) != remoteMirrorUUID {
        return conflict("Observed peer registration UUID must be nonempty and have no surrounding whitespace")
    }

    seen := make(map[string]struct{}, len(peers))

    var matched ceph.PoolMirroringPeer
    matchCount := 0
    unresolvedCount := 0

    for _, peer := range peers{
        if peer.UUID == "" || strings.TrimSpace(peer.UUID) != peer.UUID {
            return conflict("Observed peer registratin UUID must be nonempty and have no surrounding whitespace")
        }

        if _, exists := seen[peer.UUID]; exists{
            return conflict(fmt.Sprintf("peer registration UUID %q appears more than once", peer.UUID))
        }

        seen[peer.UUID] = struct{}{}

        if peer.MirrorUUID == ""{
            unresolvedCount++
            continue
        }

        if strings.TrimSpace(peer.MirrorUUID) != peer.MirrorUUID{
            return conflict(fmt.Sprintf("Peer registration %q has a mirror UUID with surrounding whitespace", peer.UUID))
        }

        if peer.MirrorUUID == remoteMirrorUUID{
            matched = peer
            matchCount++
        }
    }

    if matchCount > 1 {
        return conflict(fmt.Sprintf("%d peer registration identify remote pool mirror UUID %q", matchCount, remoteMirrorUUID))
    }

    if unresolvedCount > 0 {
        return conflict(fmt.Sprintf("%d peer registrations lack a remote mirror UUID; cannot establish an unambiguous relationship with remote pool mirror UUID %q",unresolvedCount, remoteMirrorUUID))
    }

    if matchCount == 1{
        return poolPeerIdentityResult{
            state:  poolPeerIdentityMatched,
            peer:   matched,
        }
    }

    return poolPeerIdentityResult{
        state:  poolPeerIdentityAbsent,
    }
}