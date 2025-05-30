package cluster

import "time"

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
