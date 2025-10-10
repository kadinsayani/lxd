(cluster-placement-groups)=
# How to use placement groups

Placement groups allow you to control how instances are distributed across cluster members.
You can either spread instances across different members for high availability, or compact them onto the same member(s) for performance and locality.

```{note}
Placement groups are only available in clustered LXD deployments and are scoped to individual projects.
```

## Create a placement group

Placement groups require two configuration keys: `policy` and `rigor`.

### Policy options

**Spread policy**
: Distributes instances across different cluster members to maximize availability and distribute load.

**Compact policy**
: Colocates instances on the same cluster member to minimize network latency and maximize resource sharing.

### Rigor options

**Strict rigor**
: Enforces the placement policy strictly. Instance creation fails if the policy cannot be satisfied.

**Permissive rigor**
: Attempts to follow the placement policy but allows fallback if constraints cannot be met.

### Create with spread policy

To create a placement group that spreads instances across members:

```bash
lxc placement-group create my-pg-spread policy=spread rigor=strict
```

For a permissive spread policy that allows fallback:

```bash
lxc placement-group create my-pg-spread policy=spread rigor=permissive
```

### Create with compact policy

To create a placement group that compacts instances onto the same member:

```bash
lxc placement-group create my-pg-compact policy=compact rigor=strict
```

## Assign instances to a placement group

### During instance creation

Specify the placement group when creating an instance:

```bash
lxc launch ubuntu:24.04 my-instance -c placement.group=my-pg-spread
```

### For existing instances

Add a placement group to an existing instance:

```bash
lxc config set my-instance placement.group=my-pg-spread
```

```{note}
Changing the placement group of an existing instance does not move the instance.
The new placement policy applies only to future LXD scheduling events (e.g., evacuation).
```

### Using profiles

Apply a placement group to all instances using a profile:

```bash
lxc profile set default placement.group=my-pg-spread
```

## View placement groups

List all placement groups in the current project:

```bash
lxc placement-group list
```

List placement groups from all projects:

```bash
lxc placement-group list --all-projects
```

View details of a specific placement group:

```bash
lxc placement-group show my-pg-spread
```

See which instances are using a placement group:

```bash
lxc placement-group show my-pg-spread
```

The `used_by` field shows all instances and profiles referencing this placement group.

## Modify a placement group

### Edit interactively

Open the placement group configuration in your default editor:

```bash
lxc placement-group edit my-pg-spread
```

### Update specific keys

Change the policy:

```bash
lxc placement-group set my-pg-spread policy=compact
```

Change the rigor:

```bash
lxc placement-group set my-pg-spread rigor=permissive
```

Get a configuration value:

```bash
lxc placement-group get my-pg-spread policy
```

### Add user metadata

Add custom metadata to a placement group:

```bash
lxc placement-group set my-pg-spread user.department=engineering
lxc placement-group set my-pg-spread user.cost-center=12345
```

## Rename a placement group

```bash
lxc placement-group rename my-pg-spread my-pg-ha
```

## Delete a placement group

```bash
lxc placement-group delete my-pg-spread
```

```{note}
You cannot delete a placement group that is in use. Remove it from all instances and profiles first.
```

To find what's using a placement group:

```bash
lxc placement-group show my-pg-spread | grep used_by
```

## Placement behavior

### During instance creation

When you create an instance with a placement group:

1. LXD filters cluster members according to the placement policy
2. From the filtered members, LXD selects the member with the fewest instances
3. If strict rigor is set and filtering returns no eligible members, instance creation fails
4. If permissive rigor is set and filtering returns no eligible members, LXD uses all available members

### Spread policy behavior

**Strict spread**
: - Places at most one instance per cluster member
: - Fails if there aren't enough eligible members

**Permissive spread**
: - Spreads instances as evenly as possible
: - Ensures instance count per member differs by at most one

### Compact policy behavior

**Strict compact**
: - Places all instances on the same cluster member
: - Fails if the preferred member is unavailable

**Permissive compact**
: - Prefers to place all instances on the same cluster member
: - Allows fallback to other members if the preferred member is unavailable

### During cluster evacuation

When evacuating a cluster member, LXD respects placement groups:

- **Spread policy**: Distributes evacuated instances across remaining members
- **Compact policy**: Attempts to keep instances from the same placement group together

If strict placement cannot be satisfied during evacuation, LXD falls back to the least-loaded member (unlike instance creation, which would fail).

## Examples

### High availability web servers

Create a spread placement group for web server instances:

```bash
lxc placement-group create web-servers policy=spread rigor=strict
lxc launch ubuntu:24.04 web1 -c placement.group=web-servers
lxc launch ubuntu:24.04 web2 -c placement.group=web-servers
lxc launch ubuntu:24.04 web3 -c placement.group=web-servers
```

Each web server will be placed on a different cluster member.

### Database cluster with locality

Create a compact placement group for database instances that benefit from low latency:

```bash
lxc placement-group create db-cluster policy=compact rigor=permissive
lxc launch ubuntu:24.04 db-primary -c placement.group=db-cluster
lxc launch ubuntu:24.04 db-replica1 -c placement.group=db-cluster
lxc launch ubuntu:24.04 db-replica2 -c placement.group=db-cluster
```

All database instances will preferably run on the same cluster member.

### Project isolation

Placement groups are project-scoped, so different projects can have placement groups with the same name:

```bash
lxc project create project-a
lxc project create project-b

lxc placement-group create my-pg policy=spread rigor=strict --project project-a
lxc placement-group create my-pg policy=compact rigor=strict --project project-b
```

## Troubleshooting

### Instance creation fails with strict rigor

If instance creation fails with a strict placement group:

1. Check available cluster members: `lxc cluster list`
2. Check instance distribution: `lxc list -c nL`
3. Consider using permissive rigor or adding more cluster members

### Placement group in use

If you cannot delete a placement group:

```bash
# Find what's using it
lxc placement-group show my-pg-spread

# Remove from instances
lxc config unset my-instance placement.group

# Remove from profiles
lxc profile unset my-profile placement.group

# Now delete
lxc placement-group delete my-pg-spread
```

## Related topics

- {ref}`clustering-instance-placement` - Explanation of placement group concepts
- {ref}`ref-placement-groups` - Reference documentation
- {config:option}`instance-placement:placement.group` - Instance configuration key
