package placement

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
)

// GetInstancesInPlacementGroup returns a map of instance ID to node ID for all instances that reference the given
// placement group either directly or indirectly via a profile.
func GetInstancesInPlacementGroup(ctx context.Context, tx *sql.Tx, project string, placementGroupName string) (map[int]int, error) {
	// Query notes:
	// 1. Union of profile config and instance config to perform single query. Since we plan to run this on the leader,
	//    this may not be necessary.
	// 2. The apply order must be a selected column, as we need to perform the ORDER BY to expand config correctly, and
	//    the ORDER BY can only be used after the UNION statement.
	// 3. The apply_order of 1000000 for instances is used to ensure that instance config is applied last.
	q := `
SELECT
	instances.id,
	instances.node_id,
	profiles_config.value,
	instances_profiles.apply_order AS apply_order
FROM instances 
	JOIN projects ON instances.project_id = projects.id
	JOIN instances_profiles ON instances.id = instances_profiles.instance_id 
	JOIN profiles ON instances_profiles.profile_id = profiles.id 
	JOIN profiles_config ON profiles.id = profiles_config.profile_id
WHERE projects.name = ? AND profiles_config.key = 'placement.group'
UNION
SELECT 
	instances.id,
	instances.node_id,
	instances_config.value,
	1000000 AS apply_order
FROM instances
	JOIN projects ON instances.project_id = projects.id
	JOIN instances_config ON instances.id = instances_config.instance_id 
WHERE projects.name = ? AND instances_config.key = 'placement.group'
	ORDER BY instances.id, apply_order`
	args := []any{project, project}

	// Keep a map of pointers so that each value is mutable.
	instIDToPlacementGroup := make(map[int]string)
	instIDToNodeID := make(map[int]int)
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var instID int
		var nodeID int
		var placementGroup string
		var applyOrder int
		err := scan(&instID, &nodeID, &placementGroup, &applyOrder)
		if err != nil {
			return err
		}

		instIDToNodeID[instID] = nodeID
		instIDToPlacementGroup[instID] = placementGroup
		return nil
	}, args...)
	if err != nil {
		return nil, err
	}

	result := make(map[int]int, len(instIDToPlacementGroup))
	for id, group := range instIDToPlacementGroup {
		if group == placementGroupName {
			result[id] = instIDToNodeID[id]
		}
	}

	return result, nil
}

// Filter filters the candidates using the placement group. If the runningInstanceID argument is passed, that instance
// is ignored (this is used to check if a running instance satisfies its placement group policy).
func Filter(ctx context.Context, tx *sql.Tx, candidates []db.NodeInfo, runningInstanceID *int, placementGroup cluster.PlacementGroup) ([]db.NodeInfo, error) {
	// Get the placement group config.
	apiPlacementGroup, err := placementGroup.ToAPI(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("Failed to get placement group config: %w", err)
	}

	// Get policy and rigor from config.
	policy := apiPlacementGroup.Config["policy"]
	rigor := apiPlacementGroup.Config["rigor"]

	instToNode, err := GetInstancesInPlacementGroup(ctx, tx, placementGroup.Project, placementGroup.Name)
	if err != nil {
		return nil, err
	}

	// Omit current instance from rule application.
	if runningInstanceID != nil {
		delete(instToNode, *runningInstanceID)
	}

	// Build reverse map: node ID -> instance IDs on that node.
	nodeToInst := make(map[int][]int, len(instToNode))
	for instID, nodeID := range instToNode {
		nodeToInst[nodeID] = append(nodeToInst[nodeID], instID)
	}

	var compliantCandidates []db.NodeInfo

	switch {
	case policy == api.PlacementPolicySpread && rigor == api.PlacementRigorStrict:
		// Spread + Strict: Place at most one instance per cluster member.
		// Filter out candidates that already have instances.
		for _, candidate := range candidates {
			_, hasInst := nodeToInst[int(candidate.ID)]
			if !hasInst {
				compliantCandidates = append(compliantCandidates, candidate)
			}
		}

		if len(compliantCandidates) == 0 {
			return nil, fmt.Errorf("Placement group %q policy (spread/strict) cannot be satisfied: no eligible cluster members available", placementGroup.Name)
		}

		return compliantCandidates, nil

	case policy == api.PlacementPolicySpread && rigor == api.PlacementRigorPermissive:
		// Spread + Permissive: Prefer spreading instances evenly across cluster members.
		// The number of instances per cluster member differs by at most one.

		// Find the minimum number of instances on any candidate node.
		minInstances := -1
		for _, candidate := range candidates {
			instanceCount := len(nodeToInst[int(candidate.ID)])
			if minInstances == -1 || instanceCount < minInstances {
				minInstances = instanceCount
			}
		}

		if minInstances == -1 {
			minInstances = 0
		}

		// Filter candidates to only those with at most (minInstances + 1) instances.
		// This ensures the number of instances per cluster member differs by at most one.
		for _, candidate := range candidates {
			instanceCount := len(nodeToInst[int(candidate.ID)])
			if instanceCount <= minInstances {
				compliantCandidates = append(compliantCandidates, candidate)
			}
		}

		if len(compliantCandidates) == 0 {
			return nil, fmt.Errorf("Placement group %q policy (spread/permissive) cannot be satisfied: no eligible cluster members available", placementGroup.Name)
		}

		return compliantCandidates, nil

	case policy == api.PlacementPolicyCompact && rigor == api.PlacementRigorStrict:
		// Compact + Strict: Place all instances on the same cluster member.
		// The first instance determines the cluster member.
		if len(instToNode) == 0 {
			// No instances yet.
			// All candidates are valid (first instance determines the member).
			return candidates, nil
		}

		// Find which node has instances from this placement group.
		var targetNodeID int
		for _, nodeID := range instToNode {
			targetNodeID = nodeID
			break // All instances should be on same node in compact+strict.
		}

		// Filter candidates to only include the node that already has instances.
		for _, candidate := range candidates {
			if int(candidate.ID) == targetNodeID {
				compliantCandidates = append(compliantCandidates, candidate)
				break
			}
		}

		if len(compliantCandidates) == 0 {
			return nil, fmt.Errorf("Placement group %q policy (compact/strict) cannot be satisfied: required cluster member is unavailable", placementGroup.Name)
		}

		return compliantCandidates, nil

	case policy == api.PlacementPolicyCompact && rigor == api.PlacementRigorPermissive:
		// Compact + Permissive: Prefer to place all instances on the same cluster member.
		if len(instToNode) == 0 {
			// No instances yet.
			// All candidates are valid (first instance determines preferred member).
			return candidates, nil
		}

		// Find which node has instances from this placement group.
		var preferredNodeID int
		for _, nodeID := range instToNode {
			preferredNodeID = nodeID
			break // All instances should be on same node ideally.
		}

		// Check if preferred node is in candidates.
		for _, candidate := range candidates {
			if int(candidate.ID) == preferredNodeID {
				// Preferred node is available.
				return []db.NodeInfo{candidate}, nil
			}
		}

		// Preferred node is not available - fall back to all candidates.
		return candidates, nil

	default:
		return nil, fmt.Errorf("Failed scheduling instance: Invalid placement group %q", placementGroup.Name)
	}
}
