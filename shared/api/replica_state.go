package api

const (
	// ReplicaStatusPending represents a pending replica job.
	ReplicaStatusPending = "PENDING"

	// ReplicaStatusRunning represents a running replica job.
	ReplicaStatusRunning = "RUNNING"

	// ReplicaStatusCompleted represents a completed replica job.
	ReplicaStatusCompleted = "COMPLETED"

	// ReplicaStatusFailed represents a failed replica job.
	ReplicaStatusFailed = "FAILED"
)

// ReplicaState represents the state of a replica job.
//
// swagger:model
//
// API extension: replicas.
type ReplicaState struct {
	Status string `json:"status" yaml:"status"`
}
