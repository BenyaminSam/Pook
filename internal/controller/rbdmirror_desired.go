package controller

import (
	"fmt"
	"strings"

	storagev1alpha1 "github.com/BenyaminSam/pook/api/v1alpha1"
)

type desiredPoolMirroring struct {
	name            string
	replicationMode string
	selectionPolicy string
	cephPoolMode    string
	direction       string
}

func normalizePoolMirroring(pools []storagev1alpha1.RBDMirrorPoolSpec) ([]desiredPoolMirroring, error) {
	if len(pools) == 0 {
		return nil, fmt.Errorf("At least one pool must be configured")
	}

	desired := make([]desiredPoolMirroring, 0, len(pools))
	seen := make(map[string]struct{}, len(pools))

	for _, pool := range pools {
		if pool.Name == "" || strings.TrimSpace(pool.Name) != pool.Name {
			return nil, fmt.Errorf("Pool name %q must be nonempty and have no surrounding whitespace", pool.Name)
		}

		if _, exists := seen[pool.Name]; exists {
			return nil, fmt.Errorf("Pool %q is configured more than once", pool.Name)
		}
		seen[pool.Name] = struct{}{}

		selectionPolicy := pool.ImageSelection.Policy
		if selectionPolicy == "" {
			selectionPolicy = "Pool"
		}

		direction := pool.Direction
		if direction == "" {
			direction = "rx-tx"
		}

		switch direction {
		case "rx-only", "rx-tx":
		default:
			return nil, fmt.Errorf("Pool %q has unsupported direction %q", pool.Name, direction)
		}

		var cephPoolMode string

		switch {
		case pool.Mode == "journal" && selectionPolicy == "Pool":
			cephPoolMode = "pool"
		default:
			return nil, fmt.Errorf("Pool %q has unsupported replication mode %q with image selection %q", pool.Name, pool.Mode, selectionPolicy)
		}

		desired = append(desired, desiredPoolMirroring{
			name:            pool.Name,
			replicationMode: pool.Mode,
			selectionPolicy: selectionPolicy,
			cephPoolMode:    cephPoolMode,
			direction:       direction,
		})
	}
	return desired, nil
}
