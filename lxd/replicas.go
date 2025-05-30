package main

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	dbCluster "github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/operationtype"
	"github.com/canonical/lxd/lxd/instance"
	"github.com/canonical/lxd/lxd/operations"
	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
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

	Get: APIEndpointAction{Handler: replicaGet, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanView, "name")},
	// TODO:
	// Post:   APIEndpointAction{Handler: replicaPost, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanEdit, "name")},
	// Patch:  APIEndpointAction{Handler: replicaPatch, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanEdit, "name")},
	// Put:    APIEndpointAction{Handler: replicaPut, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanEdit, "name")},
	// Delete: APIEndpointAction{Handler: replicaDelete, AccessHandler: allowPermission(entity.TypeReplica, auth.EntitlementCanDelete, "name")},
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
//      name: filter
//      description: Collection filter
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
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicasGet(d *Daemon, r *http.Request) response.Response {
	return nil
}

// swagger:operation POST /1.0/replicas replicas replicas_post
//
//	Create a new replica
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
//	      $ref: "#/definitions/ReplicasPost"
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
	s := d.State()
	projectName := request.ProjectParam(r)

	// Parse the request
	req := api.ReplicaPost{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

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
			Project: &projectName,
		})
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Get the cluster link information.
	var clusterLink *dbCluster.ClusterLink
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		clusterLink, err = dbCluster.GetClusterLink(ctx, tx.Tx(), req.TargetCluster)
		return err
	})
	if err != nil {
		return response.SmartError(fmt.Errorf("Failed to get cluster link %q: %w", req.TargetCluster, err))
	}

	// Get source client.
	client, err := lxd.ConnectLXDUnix("", nil)
	if err != nil {
		return response.SmartError(err)
	}

	// Connect to the target cluster.
	// targetClient, err := cluster.ConnectClusterLink(r.Context(), clusterLink.Addresses, s.ServerCert(), r)
	// if err != nil {
	// 	return response.SmartError(fmt.Errorf("Failed to connect to target cluster %q: %w", req.TargetCluster, err))
	// }

	clusterCert, err := util.LoadClusterCert(s.OS.VarDir)
	if err != nil {
		return response.SmartError(err)
	}

	targetCert, err := shared.GetRemoteCertificate("https://"+clusterLink.Addresses[0], version.UserAgent)
	if err != nil {
		return response.SmartError(err)
	}

	targetCertStr := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: targetCert.Raw}))

	targetClient, err := lxd.ConnectLXD("https://"+clusterLink.Addresses[0], &lxd.ConnectionArgs{
		TLSClientCert: string(clusterCert.PublicKey()),
		TLSClientKey:  string(clusterCert.PrivateKey()),
		TLSServerCert: targetCertStr,
		SkipGetServer: true,
	})
	if err != nil {
		return response.SmartError(err)
	}

	// Use the same project on the target cluster.
	targetClient = targetClient.UseProject(projectName)

	run := func(op *operations.Operation) error {
		// Copy each instance to the target cluster.
		for i, inst := range instances {
			metadata := map[string]any{
				"replica_progress": fmt.Sprintf("Copying instance %d/%d: %s", i+1, len(instances), inst.Name),
			}
			_ = op.UpdateMetadata(metadata)

			// Load the full instance to get migration capabilities.
			instProject, err := instance.LoadByProjectAndName(s, projectName, inst.Name)
			if err != nil {
				return fmt.Errorf("Failed to load instance %q: %w", inst.Name, err)
			}

			// Check if the instance can be migrated.
			_, live := instProject.CanMigrate()

			// Create copy request.
			copyReq := &lxd.InstanceCopyArgs{
				Name:    inst.Name,
				Live:    live && inst.StatusCode == api.Running,
				Mode:    "push",
				Refresh: true,
			}

			// Copy the instance to target cluster.
			copyOp, err := targetClient.CopyInstance(client, inst, copyReq)
			if err != nil {
				return fmt.Errorf("Failed to start copy of instance %q: %w", inst.Name, err)
			}

			// Wait for the copy operation to complete.
			err = copyOp.Wait()
			if err != nil {
				return fmt.Errorf("Failed to copy instance %q: %w", inst.Name, err)
			}
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

	op, err := operations.OperationCreate(s, projectName, operations.OperationClassTask, operationtype.ReplicaJob, resources, nil, run, nil, nil, r)
	if err != nil {
		return response.InternalError(err)
	}

	return operations.OperationResponse(op)
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
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func replicaGet(d *Daemon, r *http.Request) response.Response {
	return nil
}
