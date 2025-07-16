(howto-replicas)=
# How to set up replicas

Replicas are a way to sync instances across LXD cluster links. This is useful for disaster recovery or for having a read-only copy of your data at a different location.

Active-passive disaster recovery is a strategy where a primary (active) environment handles all the workload, while a secondary (passive) environment remains in standby mode, ready to take over if the primary fails. LXD supports this strategy using project replicas over a {ref}`howto-cluster-links`.

(howto-replicas-prereqs)=
## Prerequisites

Before setting up replicas:

1. Two LXD clusters should be initialized. We will call them "primary" and "standby".
1. You need sufficient permissions on both clusters to establish the links.
1. A {ref}`howto-cluster-links` must be established between the two clusters.
1. Network connectivity between the clusters.

(howto-replicas-create)=
## Create a replica

To create a new replica of a project, you need to configure a project on both the primary and standby clusters.

1. On the primary cluster, create a project and configure it to link to the standby cluster:
   ```bash
   lxc project create <project_name> -c replica.cluster_link=<standby_cluster_link_name>
   ```

1. On the standby cluster, create a project with the same name and configure it to link to the primary cluster:
   ```bash
   lxc project create <project_name> -c replica.cluster_link=<primary_cluster_link_name>
   ```

1. On the standby cluster, set the project's replica mode to `standby`. This makes the project read-only.
   ```bash
   lxc project set <project_name> replica.mode=standby
   ```

1. On the primary cluster, set the project's replica mode to `leader`.
   ```bash
   lxc project set <project_name> replica.mode=leader
   ```

1. On the primary cluster, create the replica job:
   ```bash
   lxc replica create <replica_name> --project <project_name>
   ```

(howto-replicas-run)=
## Run a replica

To manually trigger a replica run, use the following command on the primary cluster:

```bash
lxc replica run <replica_name>
```

This will sync the instances in the project from the primary cluster to the standby cluster. You can also set a schedule for the replica job, for example:

```bash
lxc replica set <replica_name> schedule="0 0 * * *"
```

(howto-replicas-view)=
## View replicas

To view all replicas, use the following command on the primary cluster:

```bash
lxc replica list
```

This shows all replicas.

To view replica configuration for a specific replica, use the following command on the primary cluster:

```bash
lxc replica show <replica_name>
```

(howto-replicas-modify)=
## Configure a replica

See {ref}`replica-config` for more details on replica configuration options.

To configure a replica, use:

```bash
lxc replica edit <replica_name>
```

Alternatively, use the `set` command to modify specific properties or configuration options:

```bash
lxc replica set <replica_name> <key> <value>
```

(howto-replicas-delete)=
## Delete a replica

To delete a replica, use:

```bash
lxc replica delete <replica_name>
```

## Disaster Recovery with Replicas

### Failover process

If your primary cluster becomes unavailable, you can perform a manual failover to the standby cluster.

To do this, on the standby cluster, promote the replica project to become the leader:

```bash
lxc project set <project_name> replica.mode=leader
```

After this command, the project on the standby cluster becomes writable, and you can start the instances within it.

### Recovering the original primary cluster

When the original primary cluster comes back online, it will be out of sync with the new primary (the former standby). Scheduled replica jobs on the original primary will fail because both projects have `replica.mode=leader`.

A replica job requires the source project to be `leader` and the target project to be `standby`.

To restore the original primary cluster and resume the original replication direction, follow these steps:

#### 1. Sync from new primary to original primary

First, sync the data from the new primary cluster (the former standby) back to the original primary cluster.

1.  On the original primary cluster, stop any running instances in the project.
1.  Set the project on the original primary cluster to standby mode:
    ```bash
    lxc project set <project_name> replica.mode=standby
    ```
1.  On the original primary cluster, run the replica job in restore mode to pull data from the new primary:
    ```bash
    lxc replica run <replica_name> --restore
    ```
The original primary cluster is now a standby replica of the new primary cluster.

#### 2. Resume original replication direction

To return to the original setup where the original primary replicates to the standby:

1.  On the new primary cluster (former standby), stop any running instances in the project.
1.  Set the project on the new primary cluster back to standby mode:
    ```bash
    lxc project set <project_name> replica.mode=standby
    ```
1.  Set the project on the original primary cluster back to leader mode:
    ```bash
    lxc project set <project_name> replica.mode=leader
    ```

Your original active-passive DR setup is now restored. You can restart your instances on the primary cluster and resume your scheduled replica jobs.

```{note}
Promoting a project to leader on a source cluster is only possible when `replica.mode` is set to `standby` on the target cluster.
This ensures that new instances are not created on a target cluster between replica runs.
```
