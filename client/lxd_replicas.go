package lxd

import (
	"net/http"

	"github.com/canonical/lxd/shared/api"
)

// GetReplicas returns all replicas.
func (r *ProtocolLXD) GetReplicas() ([]api.Replica, error) {
	err := r.CheckExtension("replicas")
	if err != nil {
		return nil, err
	}

	replicas := []api.Replica{}
	_, err = r.queryStruct(http.MethodGet, api.NewURL().Path("replicas").WithQuery("recursion", "1").String(), nil, "", &replicas)
	if err != nil {
		return nil, err
	}

	return replicas, nil
}

// GetReplica returns information about a replica.
func (r *ProtocolLXD) GetReplica(name string) (*api.Replica, string, error) {
	err := r.CheckExtension("replicas")
	if err != nil {
		return nil, "", err
	}

	replica := &api.Replica{}
	etag, err := r.queryStruct(http.MethodGet, api.NewURL().Path("replicas", name).String(), nil, "", &replica)
	if err != nil {
		return nil, "", err
	}

	return replica, etag, nil
}

// GetReplicaNames returns replica names.
func (r *ProtocolLXD) GetReplicaNames() ([]string, error) {
	err := r.CheckExtension("replicas")
	if err != nil {
		return nil, err
	}

	urls := []string{}
	baseURL := api.NewURL().Path("replicas").String()
	_, err = r.queryStruct(http.MethodGet, baseURL, nil, "", &urls)
	if err != nil {
		return nil, err
	}

	// Parse it.
	return urlsToResourceNames(baseURL, urls...)
}

// CreateReplica requests to create a new replica.
func (r *ProtocolLXD) CreateReplica(replica api.ReplicaPost) error {
	err := r.CheckExtension("replicas")
	if err != nil {
		return err
	}

	_, _, err = r.query(http.MethodPost, api.NewURL().Path("replicas").String(), replica, "")
	if err != nil {
		return err
	}

	return nil
}

// RunReplica runs a replica.
func (r *ProtocolLXD) RunReplica(replica api.ReplicaPost) error {
	err := r.CheckExtension("replicas")
	if err != nil {
		return err
	}

	_, _, err = r.query(http.MethodPost, api.NewURL().Path("replicas", replica.Name).String(), replica, "")
	if err != nil {
		return err
	}

	return nil
}

// UpdateReplica updates a replica.
func (r *ProtocolLXD) UpdateReplica(name string, replica api.ReplicaPut, ETag string) error {
	err := r.CheckExtension("replicas")
	if err != nil {
		return err
	}

	_, _, err = r.query(http.MethodPut, api.NewURL().Path("replicas", name).String(), replica, ETag)
	if err != nil {
		return err
	}

	return nil
}

// DeleteReplica deletes a replica.
func (r *ProtocolLXD) DeleteReplica(name string) error {
	err := r.CheckExtension("replicas")
	if err != nil {
		return err
	}

	_, _, err = r.query(http.MethodDelete, api.NewURL().Path("replicas", name).String(), nil, "")
	if err != nil {
		return err
	}

	return nil
}

// GetReplicaState gets state information about a replica.
func (r *ProtocolLXD) GetReplicaState(name string) (*api.ReplicaState, string, error) {
	err := r.CheckExtension("replicas")
	if err != nil {
		return nil, "", err
	}

	state := api.ReplicaState{}
	u := api.NewURL().Path("replicas", name, "state")
	etag, err := r.queryStruct(http.MethodGet, u.String(), nil, "", &state)
	if err != nil {
		return nil, "", err
	}

	return &state, etag, err
}
