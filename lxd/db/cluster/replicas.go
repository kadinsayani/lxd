package cluster

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
)

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t replicas.mapper.go
//go:generate mapper reset -i -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e replica objects
//go:generate mapper stmt -e replica objects-by-Name
//go:generate mapper stmt -e replica objects-by-ID
//go:generate mapper stmt -e replica create
//go:generate mapper stmt -e replica id
//go:generate mapper stmt -e replica rename
//go:generate mapper stmt -e replica update
//go:generate mapper stmt -e replica delete-by-Name
//
//go:generate mapper method -i -e replica GetMany
//go:generate mapper method -i -e replica GetOne
//go:generate mapper method -i -e replica ID
//go:generate mapper method -i -e replica Exists
//go:generate mapper method -i -e replica Create
//go:generate mapper method -i -e replica DeleteOne-by-Name
//go:generate mapper method -i -e replica Update
//go:generate mapper method -i -e replica Rename
//go:generate goimports -w replicas.mapper.go
//go:generate goimports -w replicas.interface.mapper.go

// Replica is the database representation of an [api.Replica].
type Replica struct {
	ID            int
	ClusterLinkID int
	ProjectID     int
	Name          string `db:"primary=yes"`
	Description   string `db:"coalesce=''"`
	LastRunDate   time.Time
}

// ReplicaFilter contains fields upon which a replica can be filtered.
type ReplicaFilter struct {
	ID   *int
	Name *string
}

// ToAPI converts the database [Replica] struct to API type [api.Replica].
func (r *Replica) ToAPI(ctx context.Context, tx *sql.Tx) (*api.Replica, error) {
	replica := &api.Replica{
		Name:        r.Name,
		Description: r.Description,
		LastRunAt:   r.LastRunDate,
	}

	project, err := GetProjectName(ctx, tx, r.ProjectID)
	if err != nil {
		return nil, err
	}

	replica.Project = project

	err = getReplicaConfig(ctx, tx, int64(r.ID), replica)
	if err != nil {
		return nil, err
	}

	return replica, nil
}

// CreateReplicaConfig creates config for a new replica with the given name.
func CreateReplicaConfig(ctx context.Context, tx *sql.Tx, name string, config map[string]string) error {
	id, err := GetReplicaID(ctx, tx, name)
	if err != nil {
		return err
	}

	err = replicaConfigAdd(tx, id, config)
	if err != nil {
		return err
	}

	return nil
}

// UpdateReplicaConfig updates the replica with the given name, setting its config.
func UpdateReplicaConfig(ctx context.Context, tx *sql.Tx, name string, config map[string]string) error {
	id, err := GetReplicaID(ctx, tx, name)
	if err != nil {
		return err
	}

	err = clearReplicaConfig(tx, id)
	if err != nil {
		return err
	}

	err = replicaConfigAdd(tx, id, config)
	if err != nil {
		return err
	}

	return nil
}

// getReplicaConfig populates the config map of the [api.Replica] with the given ID.
func getReplicaConfig(ctx context.Context, tx *sql.Tx, replicaID int64, replica *api.Replica) error {
	q := `
        SELECT key, value
        FROM replicas_config
		WHERE replica_id=?
	`

	replica.Config = map[string]string{}

	return query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var key, value string

		err := scan(&key, &value)
		if err != nil {
			return err
		}

		_, found := replica.Config[key]
		if found {
			return fmt.Errorf("Duplicate config row found for key %q for replica ID %d", key, replicaID)
		}

		replica.Config[key] = value

		return nil
	}, replicaID)
}

// replicaConfigAdd adds config to the replica with the given ID.
func replicaConfigAdd(tx *sql.Tx, replicaID int64, config map[string]string) error {
	str := "INSERT INTO replicas_config (replica_id, key, value) VALUES(?, ?, ?)"
	stmt, err := tx.Prepare(str)
	if err != nil {
		return err
	}

	defer func() { _ = stmt.Close() }()

	for k, v := range config {
		if v == "" {
			continue
		}

		_, err = stmt.Exec(replicaID, k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// clearReplicaConfig removes any config from the replica with the given ID.
func clearReplicaConfig(tx *sql.Tx, replicaID int64) error {
	_, err := tx.Exec(
		"DELETE FROM replicas_config WHERE replica_id=?", replicaID)
	if err != nil {
		return err
	}

	return nil
}
