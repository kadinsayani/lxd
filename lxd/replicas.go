package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/cluster"
	"github.com/canonical/lxd/lxd/db"
	dbCluster "github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/operationtype"
	"github.com/canonical/lxd/lxd/instance"
	"github.com/canonical/lxd/lxd/lifecycle"
	"github.com/canonical/lxd/lxd/operations"
	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/state"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/logger"
	"github.com/canonical/lxd/shared/validate"
)

var replicasCmd = APIEndpoint{
	Path:        "replicas",
	MetricsType: entity.TypeReplica,

	Get:  APIEndpointAction{Handler: replicasGet, AccessHandler: allowAuthenticated},
	Post: APIEndpointAction{Handler: replicasPost, AccessHandler: allowPermission(entity.TypeProject, auth.EntitlementAdmin)},
}

var replicaCmd = APIEndpoint{
	Path:        "replicas/{name}",
	MetricsType: entity.TypeReplica,

	Get:    APIEndpointAction{Handler: replicaGet, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanView, "name")},
	Post:   APIEndpointAction{Handler: replicaPost, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanEdit, "name")},
	Patch:  APIEndpointAction{Handler: replicaPatch, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanEdit, "name")},
	Put:    APIEndpointAction{Handler: replicaPut, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanEdit, "name")},
	Delete: APIEndpointAction{Handler: replicaDelete, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanDelete, "name")},
}

var replicaStateCmd = APIEndpoint{
	Path:        "replicas/{name}/state",
	MetricsType: entity.TypeReplica,

	Get: APIEndpointAction{Handler: replicaStateGet, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanView, "name")},
}

// swagger:operation GET /1.0/replicas replicas replicas_get
//
//  Get the replicas
//
//  Returns a list of replicas (URLs).
//
//  ---
//  produces:
//    - application/json
//  parameters:
//    - in: query
//      name: project
//      description: Project name
//      type: string
//      example: default
//    - in: query
//      name: all-projects
//      description: Retrieve replicas from all projects
//      type: boolean
//  responses:
//    "200":
//      description: API endpoints
//      schema:
//        type: object
//        description: Sync response
//        properties:
//          type:
//            type: string
//            description: Response type
//            example: sync
//          status:
//            type: string
//            description: Status description
//            example: Success
//          status_code:
//            type: integer
//            description: Status code
//            example: 200
//          metadata:
//            type: array
//            description: List of endpoints
//            items:
//              type: string
//            example: |-
//              [
//                "/1.0/replicas/foo",
//                "/1.0/replicas/bar"
//              ]
//    "400":
//      $ref: "#/responses/BadRequest"
//    "403":
//      $ref: "#/responses/Forbidden"
//    "500":
//      $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/replicas?recursion=1 replicas replicas_get_recursion1
//
//	Get the replicas
//
//	Returns a list of replicas (structs).
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	  - in: query
//	    name: all-projects
//	    description: Retrieve replicas from all projects
//	    type: boolean
//	responses:
//	  "200":
//	    description: API endpoints
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of replicas
//	          items:
//	            $ref: "#/definitions/Replica"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicasGet(d *Daemon, r *http.Request) response.Response {
	// TODO: Handle all-projects requests.
	projectName, _, err := request.ProjectParams(r)
	if err != nil {
		return response.SmartError(err)
	}

	recursion := util.IsRecursionRequest(r)
	withEntitlements, err := extractEntitlementsFromQuery(r, entity.TypeReplica, true)
	if err != nil {
		return response.SmartError(err)
	}

	s := d.State()

	userHasPermission, err := s.Authorizer.GetPermissionChecker(r.Context(), auth.EntitlementCanView, entity.TypeReplica)
	if err != nil {
		return response.InternalError(err)
	}

	var apiReplicas []*api.Replica
	var replicaURLs []string
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		allReplicas, err := dbCluster.GetReplicas(ctx, tx.Tx())
		if err != nil {
			return fmt.Errorf("Failed to fetch replicas: %w", err)
		}

		replicas := make([]dbCluster.Replica, 0, len(allReplicas))
		for _, replica := range allReplicas {
			if userHasPermission(entity.ReplicaURL(projectName, replica.Name)) {
				replicas = append(replicas, replica)
			}
		}

		if recursion {
			apiReplicas = make([]*api.Replica, 0, len(replicas))
			for _, replica := range replicas {
				apiReplica, err := replica.ToAPI(ctx, tx.Tx())
				if err != nil {
					return err
				}

				apiReplicas = append(apiReplicas, apiReplica)
			}
		} else {
			replicaURLs = make([]string, 0, len(replicas))
			for _, replica := range replicas {
				replicaURLs = append(replicaURLs, entity.ReplicaURL(projectName, replica.Name).String())
			}
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	if !recursion {
		return response.SyncResponse(true, replicaURLs)
	}

	if len(withEntitlements) > 0 {
		urlToReplica := make(map[*api.URL]auth.EntitlementReporter, len(apiReplicas))
		for _, replica := range apiReplicas {
			u := entity.ClusterLinkURL(replica.Name)
			urlToReplica[u] = replica
		}

		err = reportEntitlements(r.Context(), s.Authorizer, s.IdentityCache, entity.TypeReplica, withEntitlements, urlToReplica)
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.SyncResponse(true, apiReplicas)
}

// swagger:operation GET /1.0/replicas/{name} replicas replicas_get
//
//	Get the replica
//
//	Gets a specific replica (struct).
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	responses:
//	  "200":
//	    description: Replica
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/Replica"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicaGet(d *Daemon, r *http.Request) response.Response {
	projectName, _, err := request.ProjectParams(r)
	if err != nil {
		return response.SmartError(err)
	}

	withEntitlements, err := extractEntitlementsFromQuery(r, entity.TypeReplica, false)
	if err != nil {
		return response.SmartError(err)
	}

	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	s := d.State()

	var apiReplica *api.Replica
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		dbReplica, err := dbCluster.GetReplica(ctx, tx.Tx(), name)
		if err != nil {
			return fmt.Errorf("Failed to fetch replica: %w", err)
		}

		apiReplica, err = dbReplica.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	if len(withEntitlements) > 0 {
		err = reportEntitlements(r.Context(), s.Authorizer, s.IdentityCache, entity.TypeReplica, withEntitlements, map[*api.URL]auth.EntitlementReporter{entity.ReplicaURL(projectName, name): apiReplica})
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.SyncResponseETag(true, apiReplica, apiReplica.Writable())
}

// replicaValidateConfig validates the configuration keys/values for replicas.
func replicaValidateConfig(config map[string]string) error {
	replicaConfigKeys := map[string]func(value string) error{
		// lxdmeta:generate(entities=replica; group=conf; key=snapshot)
		//
		// ---
		//  type: bool
		//  shortdesc: Whether to take snapshots of the project's instances before replication or not.
		//  scope: global
		"snapshot": validate.Optional(validate.IsBool),
	}

	for k, v := range config {
		// User keys are free for all.

		// lxdmeta:generate(entities=replica; group=conf; key=user.*)
		// User keys can be used in search.
		// ---
		//  type: string
		//  shortdesc: Free form user key/value storage
		if strings.HasPrefix(k, "user.") {
			continue
		}

		validator, ok := replicaConfigKeys[k]
		if !ok {
			return fmt.Errorf("Invalid replica configuration key %q", k)
		}

		err := validator(v)
		if err != nil {
			return fmt.Errorf("Invalid replica configuration key %q value", k)
		}
	}

	return nil
}

// swagger:operation POST /1.0/replicas/{name} replicas replica_post
//
//	Run the replica
//
//	Runs the replica.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: body
//	    name: replica
//	    description: Replica request
//	    required: false
//	    schema:
//	      $ref: "#/definitions/ReplicaPost"
//	responses:
//	  "202":
//	    $ref: "#/responses/Operation"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicaPost(d *Daemon, r *http.Request) response.Response {
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	req := api.ReplicaPost{}
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	s := d.State()

	// Get the replica information.
	var replica *api.Replica
	var replicaID int64
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		var dbReplica *dbCluster.Replica
		dbReplica, err = dbCluster.GetReplica(ctx, tx.Tx(), name)
		if err != nil {
			return fmt.Errorf("Failed to fetch replica %q: %w", name, err)
		}

		replicaID = int64(dbReplica.ID)

		replica, err = dbReplica.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Check source cluster project exists.
	var sourceClusterProject *api.Project
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		// Get project.
		dbSourceClusterProject, err := dbCluster.GetProject(ctx, tx.Tx(), replica.Project)
		if err != nil {
			return err
		}

		sourceClusterProject, err = dbSourceClusterProject.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get project %q: %w", replica.Project, err))
	}

	targetClusterLink := sourceClusterProject.Config["replica.cluster_link"]
	if targetClusterLink == "" {
		return response.BadRequest(fmt.Errorf("Replica %q does not have replica.cluster_link set", name))
	}

	// Get the cluster link information.
	var clusterLink *api.ClusterLink
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		var dbClusterLink *dbCluster.ClusterLink
		dbClusterLink, err = dbCluster.GetClusterLink(ctx, tx.Tx(), targetClusterLink)
		if err != nil {
			return fmt.Errorf("Failed to fetch cluster link %q: %w", targetClusterLink, err)
		}

		clusterLink, err = dbClusterLink.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Get source client.
	client, err := lxd.ConnectLXDUnix("", nil)
	if err != nil {
		return response.SmartError(err)
	}

	client = client.UseProject(replica.Project)

	// Connect to the target cluster.
	targetClient, err := cluster.ConnectClusterLink(r.Context(), s, *clusterLink)
	if err != nil {
		return response.SmartError(err)
	}

	targetClusterProject, _, err := targetClient.GetProject(sourceClusterProject.Name)
	if err != nil {
		return response.SmartError(err)
	}

	// Check "replica.mode".
	if sourceClusterProject.Config["replica.mode"] == "leader" && targetClusterProject.Config["replica.mode"] == "leader" {
		return response.BadRequest(errors.New("Source and target cluster projects cannot both be leaders"))
	} else if sourceClusterProject.Config["replica.mode"] == "leader" && targetClusterProject.Config["replica.mode"] != "standby" {
		return response.BadRequest(fmt.Errorf("Target cluster project %q must have replica.mode set to 'standby'", targetClusterProject.Name))
	} else if sourceClusterProject.Config["replica.mode"] == "" || targetClusterProject.Config["replica.mode"] == "" {
		return response.BadRequest(errors.New("Source and target cluster projects must have replica.mode"))
	}

	// TODO: Check "replica.cluster_link" on target. Or this can go in instance post.

	// Get all instances in the specified project.
	var instances []api.Instance
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		return tx.InstanceList(ctx, func(inst db.InstanceArgs, project api.Project) error {
			apiInstance, err := inst.ToAPI()
			if err != nil {
				return err
			}

			instances = append(instances, *apiInstance)
			return nil
		}, dbCluster.InstanceFilter{
			Project: &replica.Project,
		})
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Use the same project on the target cluster.
	targetClient = targetClient.UseProject(replica.Project)
	logger.Debug("Using target project", logger.Ctx{"project": replica.Project})

	run := func(op *operations.Operation) error {
		err := s.DB.Cluster.Transaction(context.TODO(), func(ctx context.Context, tx *db.ClusterTx) error {
			err = dbCluster.UpdateReplicaLastRunDate(tx.Tx(), replicaID, time.Now().UTC())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		if shared.IsTrue(replica.Config["snapshot"]) {
			// Snapshot each instance.
			for i, inst := range instances {
				metadata := map[string]any{
					"replica_progress": fmt.Sprintf("Snapshotting instance %d/%d: %s", i+1, len(instances), inst.Name),
				}
				_ = op.UpdateMetadata(metadata)

				_, err := client.CreateInstanceSnapshot(inst.Name, api.InstanceSnapshotsPost{Name: ""})
				if err != nil {
					return err
				}
			}
		}

		// Copy each instance to the target cluster.
		var wg sync.WaitGroup
		errs := make(chan error, len(instances))

		for i, inst := range instances {
			wg.Add(1)
			go func(i int, inst api.Instance) {
				defer wg.Done()

				metadata := map[string]any{
					"replica_progress": fmt.Sprintf("Copying instance %d/%d: %s", i+1, len(instances), inst.Name),
				}
				_ = op.UpdateMetadata(metadata)

				// Load the full instance to get migration capabilities.
				instProject, err := instance.LoadByProjectAndName(s, replica.Project, inst.Name)
				if err != nil {
					errs <- fmt.Errorf("Failed to load instance %q: %w", inst.Name, err)
					return
				}

				// Check if the instance can be migrated.
				_, live := instProject.CanMigrate()

				// Create copy request (incremental refresh).
				copyReq := &lxd.InstanceCopyArgs{
					Name:    inst.Name,
					Live:    live,
					Mode:    "push",
					Refresh: true,
					Replica: true,
				}

				// Copy the instance to target cluster.
				if req.Restore {
					copyOp, err := client.CopyInstance(targetClient, inst, copyReq)
					if err != nil {
						errs <- fmt.Errorf("Failed to start copy of instance %q: %w", inst.Name, err)
						return
					}

					// Wait for the copy operation to complete.
					err = copyOp.Wait()
					if err != nil {
						errs <- fmt.Errorf("Failed to copy instance %q: %w", inst.Name, err)
						return
					}
				} else {
					copyOp, err := targetClient.CopyInstance(client, inst, copyReq)
					if err != nil {
						errs <- fmt.Errorf("Failed to start copy of instance %q: %w", inst.Name, err)
						return
					}

					// Wait for the copy operation to complete.
					err = copyOp.Wait()
					if err != nil {
						errs <- fmt.Errorf("Failed to copy instance %q: %w", inst.Name, err)
						return
					}
				}
			}(i, inst)
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			return err
		}

		return nil
	}

	// Create and start the operation.
	resources := map[string][]api.URL{
		"instances": make([]api.URL, len(instances)),
	}

	for i, instance := range instances {
		resources["instances"][i] = *api.NewURL().Path("instances", instance.Name)
	}

	op, err := operations.OperationCreate(context.TODO(), s, replica.Project, operations.OperationClassTask, operationtype.ReplicaRun, resources, nil, run, nil, nil)
	if err != nil {
		return response.InternalError(err)
	}

	return operations.OperationResponse(op)
}

// swagger:operation POST /1.0/replicas replicas replicas_post
//
//	Add a replica
//
//	Creates a new replica.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	  - in: body
//	    name: replica
//	    description: Replica request
//	    required: false
//	    schema:
//	      $ref: "#/definitions/ReplicaPost"
//	responses:
//	  "202":
//	    $ref: "#/responses/Operation"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicasPost(d *Daemon, r *http.Request) response.Response {
	// Parse the request.
	req := api.ReplicaPost{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	s := d.State()

	// Check source cluster project exists.
	var sourceClusterProject *api.Project
	var sourceClusterProjectID int
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		// Get project.
		dbSourceClusterProject, err := dbCluster.GetProject(ctx, tx.Tx(), req.Project)
		if err != nil {
			return err
		}

		sourceClusterProjectID = dbSourceClusterProject.ID

		sourceClusterProject, err = dbSourceClusterProject.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get project %q: %w", req.Project, err))
	}

	// Check source cluster project config.
	if sourceClusterProject.Config["replica.mode"] != "leader" {
		return response.BadRequest(fmt.Errorf("Source cluster project %q must have replica.mode set to 'leader'", req.Project))
	}

	// Get the cluster link information.
	var clusterLink *api.ClusterLink
	targetClusterLink := sourceClusterProject.Config["replica.cluster_link"]
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		var dbClusterLink *dbCluster.ClusterLink
		dbClusterLink, err = dbCluster.GetClusterLink(ctx, tx.Tx(), targetClusterLink)
		if err != nil {
			return fmt.Errorf("Failed to fetch cluster link %q: %w", targetClusterLink, err)
		}

		clusterLink, err = dbClusterLink.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Connect to the target cluster.
	targetClient, err := cluster.ConnectClusterLink(r.Context(), s, *clusterLink)
	if err != nil {
		return response.SmartError(err)
	}

	// Check target cluster project config.
	targetClusterProject, _, err := targetClient.GetProject(sourceClusterProject.Name)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Failed to get target cluster project %q: %w", sourceClusterProject.Name, err))
	}

	if targetClusterProject.Config["replica.mode"] != "standby" {
		return response.BadRequest(fmt.Errorf("Target cluster project %q must have replica.mode set to 'standby'", sourceClusterProject.Name))
	}

	// Create the replica.
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		err := replicaValidateConfig(req.Config)
		if err != nil {
			return err
		}

		// Get cluster link ID (if exists).
		clusterLinkID, err := dbCluster.GetClusterLinkID(ctx, tx.Tx(), targetClusterLink)
		if err != nil {
			return err
		}

		_, err = dbCluster.CreateReplica(ctx, tx.Tx(), dbCluster.Replica{
			Name:          req.Name,
			Description:   req.Description,
			ProjectID:     sourceClusterProjectID,
			ClusterLinkID: int(clusterLinkID),
		})
		if err != nil {
			return fmt.Errorf("Error inserting %q into database: %w", req.Name, err)
		}

		err = dbCluster.CreateReplicaConfig(ctx, tx.Tx(), req.Name, req.Config)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.InternalError(fmt.Errorf("Failed to create replica: %w", err))
	}

	// Send replica lifecycle event.
	requestor := request.CreateRequestor(r.Context())
	s.Events.SendLifecycle(api.ProjectDefaultName, lifecycle.ReplicaCreated.Event(req.Name, req.Project, requestor, nil))

	return response.EmptySyncResponse
}

// updateReplica is shared between [replicaPut] and [replicaPatch].
func updateReplica(s *state.State, r *http.Request, isPatch bool) response.Response {
	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	var dbReplica *dbCluster.Replica
	var apiReplica *api.Replica
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		// Get replica by name.
		dbReplica, err = dbCluster.GetReplica(ctx, tx.Tx(), name)
		if err != nil {
			return fmt.Errorf("Failed to fetch replica: %w", err)
		}

		apiReplica, err = dbReplica.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Validate ETag.
	err = util.EtagCheck(r, apiReplica.Writable())
	if err != nil {
		return response.PreconditionFailed(err)
	}

	// Parse the request.
	req := api.ReplicaPut{}
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		// Update the fields from the request.
		err = replicaValidateConfig(req.Config)
		if err != nil {
			return err
		}

		if isPatch {
			// Populate request config with current values.
			if req.Config == nil {
				req.Config = apiReplica.Config
			} else {
				for k, v := range apiReplica.Config {
					_, ok := req.Config[k]
					if !ok {
						req.Config[k] = v
					}
				}
			}
		}

		if req.Description != "" {
			dbReplica.Description = req.Description
		}

		err = dbCluster.UpdateReplica(ctx, tx.Tx(), name, *dbReplica)
		if err != nil {
			return err
		}

		err = dbCluster.UpdateReplicaConfig(ctx, tx.Tx(), apiReplica.Name, req.Config)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Send replica lifecycle event.
	requestor := request.CreateRequestor(r.Context())
	s.Events.SendLifecycle(api.ProjectDefaultName, lifecycle.ReplicaUpdated.Event(name, request.ProjectParam(r), requestor, nil))

	return response.SyncResponse(true, name)
}

// swagger:operation PATCH /1.0/replicas/{name} replicas replica_patch
//
//	Update the replica
//
//	Updates a subset of the replica configuration.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Update replica request
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/ReplicaPut"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicaPatch(d *Daemon, r *http.Request) response.Response {
	return updateReplica(d.State(), r, true)
}

// swagger:operation PUT /1.0/cluster/links/{name} replicas replica_put
//
//	Update the replica
//
//	Updates the replica configuration.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Update replica request
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/ReplicaPut"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicaPut(d *Daemon, r *http.Request) response.Response {
	return updateReplica(d.State(), r, false)
}

// swagger:operation DELETE /1.0/replicas/{name} replicas replica_delete
//
//	Delete the replica
//
//	Deletes the replica.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicaDelete(d *Daemon, r *http.Request) response.Response {
	s := d.State()

	name, err := url.PathUnescape(mux.Vars(r)["name"])
	if err != nil {
		return response.SmartError(err)
	}

	// Update DB entry.
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		err = dbCluster.DeleteReplica(ctx, tx.Tx(), name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Error deleting %q from database: %w", name, err))
	}

	// Send replica lifecycle event.
	requestor := request.CreateRequestor(r.Context())
	s.Events.SendLifecycle(api.ProjectDefaultName, lifecycle.ReplicaRemoved.Event(name, request.ProjectParam(r), requestor, nil))

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/replicas/{name}/state replicas replica_state_get
//
//	Get the replica state
//
//	Get a specific replcia state.
//
//	---
//	produces:
//	  - application/json
//	responses:
//	  "200":
//	    description: Replcia state
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/ReplicaState"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicaStateGet(d *Daemon, r *http.Request) response.Response {
	return nil
}
