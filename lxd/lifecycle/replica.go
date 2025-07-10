package lifecycle

import (
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/version"
)

// ReplicaAction represents a lifecycle event action for replicas.
type ReplicaAction string

// All supported lifecycle events for replicas.
const (
	ReplicaCreated = ReplicaAction(api.EventLifecycleReplicaCreated)
	ReplicaRemoved = ReplicaAction(api.EventLifecycleReplicaRemoved)
	ReplicaUpdated = ReplicaAction(api.EventLifecycleReplicaUpdated)
	ReplicaRenamed = ReplicaAction(api.EventLifecycleReplicaRenamed)
)

// Event creates the lifecycle event for an action on a replica.
func (a ReplicaAction) Event(name string, projectName string, requestor *api.EventLifecycleRequestor, ctx map[string]any) api.EventLifecycle {
	u := api.NewURL().Path(version.APIVersion, "replicas", name).Project(projectName)

	return api.EventLifecycle{
		Action:    string(a),
		Source:    u.String(),
		Context:   ctx,
		Requestor: requestor,
	}
}
