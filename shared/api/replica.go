package api

import "time"

// Replica represents high-level information about a replica.
//
// swagger:model
//
// API extension: replicas.
type Replica struct {
	WithEntitlements `yaml:",inline"`

	// The name of the replica
	// Example: lxd02
	Name string `json:"name" yaml:"name"`

	// Description of the replica
	// Example: Backup LXD cluster
	Description string `json:"description" yaml:"description"`

	// The source project
	// Example: default
	Project string `json:"project" yaml:"project"`

	// Replica configuration map (refer to doc/replicas.md)
	// Example: {"schedule": "@daily"}
	Config map[string]string `json:"config" yaml:"config"`

	// When the replica job was last run (independent of status).
	// Example: 2021-03-23T17:38:37.753398689-04:00
	LastRunAt time.Time `json:"last_run_at" yaml:"last_run_at"`
}

// ReplicaPut represents the modifiable fields of a replica.
//
// swagger:model
//
// API extension: replicas.
type ReplicaPut struct {
	// Replica configuration map (refer to doc/replicas.md)
	// Example: {"schedule": "@daily"}
	Config map[string]string `json:"config" yaml:"config"`

	// Description of the replica
	// Example: My new replica
	Description string `json:"description" yaml:"description"`
}

// ReplicaPost represents the fields available for a new replica.
//
// swagger:model
//
// API extension: replicas.
type ReplicaPost struct {
	ReplicaPut `yaml:",inline"`

	// The name of the replica
	// Example: backup-lxd02
	Name string `json:"name" yaml:"name"`

	// The source project
	// Example: default
	Project string `json:"project" yaml:"project"`

	// Restore mode
	// Example: true
	Restore bool `json:"restore" yaml:"restore"`
}

// Writable converts a full Replica struct into a [ReplicaPut] struct (filters read-only fields).
func (replica *Replica) Writable() ReplicaPut {
	return ReplicaPut{
		Description: replica.Description,
		Config:      replica.Config,
	}
}
