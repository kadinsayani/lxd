//go:build linux && cgo && !agent

package cluster

import (
	"context"
	"database/sql"
)

// ReplicaGenerated is an interface of generated methods for Replica.
type ReplicaGenerated interface {
	// GetReplicas returns all available replicas.
	// generator: replica GetMany
	GetReplicas(ctx context.Context, tx *sql.Tx, filters ...ReplicaFilter) ([]Replica, error)

	// GetReplica returns the replica with the given key.
	// generator: replica GetOne
	GetReplica(ctx context.Context, tx *sql.Tx, name string) (*Replica, error)

	// GetReplicaID return the ID of the replica with the given key.
	// generator: replica ID
	GetReplicaID(ctx context.Context, tx *sql.Tx, name string) (int64, error)

	// ReplicaExists checks if a replica with the given key exists.
	// generator: replica Exists
	ReplicaExists(ctx context.Context, tx *sql.Tx, name string) (bool, error)

	// CreateReplica adds a new replica to the database.
	// generator: replica Create
	CreateReplica(ctx context.Context, tx *sql.Tx, object Replica) (int64, error)

	// DeleteReplica deletes the replica matching the given key parameters.
	// generator: replica DeleteOne-by-Name
	DeleteReplica(ctx context.Context, tx *sql.Tx, name string) error

	// UpdateReplica updates the replica matching the given key parameters.
	// generator: replica Update
	UpdateReplica(ctx context.Context, tx *sql.Tx, name string, object Replica) error

	// RenameReplica renames the replica matching the given key parameters.
	// generator: replica Rename
	RenameReplica(ctx context.Context, tx *sql.Tx, name string, to string) error
}
