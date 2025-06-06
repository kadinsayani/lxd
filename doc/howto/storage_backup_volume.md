---
myst:
  substitutions:
    type: "volume"
---

(howto-storage-backup-volume)=
# How to back up custom storage volumes

There are different ways of backing up your custom storage volumes:

- {ref}`storage-backup-snapshots`
- {ref}`storage-backup-export`
- {ref}`storage-copy-volume`

<!-- Include start backup types -->
Which method to choose depends both on your use case and on the storage driver you use.

In general, snapshots are quick and space efficient (depending on the storage driver), but they are stored in the same storage pool as the {{type}} and therefore not too reliable.
Export files can be stored on different disks and are therefore more reliable.
They can also be used to restore the {{type}} into a different storage pool.
If you have a separate, network-connected LXD server available, regularly copying {{type}}s to this other server gives high reliability as well, and this method can also be used to back up snapshots of the {{type}}.
<!-- Include end backup types -->

```{note}
Custom storage volumes might be attached to an instance, but they are not part of the instance.
Therefore, the content of a custom storage volume is not stored when you {ref}`back up your instance <instances-backup>`.
You must back up the data of your storage volume separately.
```

(storage-backup-snapshots)=
## Use snapshots for volume backup

A snapshot saves the state of the storage volume at a specific time, which makes it easy to restore the volume to a previous state.
It is stored in the same storage pool as the volume itself.

<!-- Include start optimized snapshots -->
Most storage drivers support optimized snapshot creation (see {ref}`storage-drivers-features`).
For these drivers, creating snapshots is both quick and space-efficient.
For the `dir` driver, snapshot functionality is available but not very efficient.
For the `lvm` driver, snapshot creation is quick, but restoring snapshots is efficient only when using thin-pool mode.
<!-- Include end optimized snapshots -->

### Create a snapshot of a custom storage volume

`````{tabs}
````{group-tab} CLI

Use the following command to create a snapshot for a custom storage volume:

    lxc storage volume snapshot <pool_name> <volume_name> [<snapshot_name>]

<!-- Include start create snapshot options -->
The snapshot name is optional.
If you don't specify one, the name follows the naming pattern defined in `snapshots.pattern`.

Add the `--reuse` flag in combination with a snapshot name to replace an existing snapshot.

By default, snapshots are kept forever, unless the `snapshots.expiry` configuration option is set.
To retain a specific snapshot even if a general expiry time is set, use the `--no-expiry` flag.
<!-- Include end create snapshot options -->

````
```` {group-tab} UI

To create a snapshot of a custom storage volume, navigate to the {guilabel}`Snapshots` tab for the target volume and click {guilabel}`Create snapshot`.

```{figure} /images/storage/storage_volumes_snapshots_tab.png
:width: 80%
:alt: LXD Storage Volumes - Snapshots tab
```

In the modal that appears, you can provide the snapshot with a name and expiry date and time. If the name is left blank, a name is automatically assigned to the snapshot based on the global snapshot configuration. If the expiry date and time are left blank, the snapshot does not expire.

```{figure} /images/storage/storage_volumes_snapshots_create.png
:width: 60%
:alt: LXD Storage Volumes - Create snapshot
```

````
`````

(storage-edit-snapshots)=
### View, edit or delete snapshots

`````{tabs}
````{group-tab} CLI

Use the following command to display the snapshots for a storage volume:

    lxc storage volume info <pool_name> <volume_name>

You can view or modify snapshots in a similar way to custom storage volumes, by referring to the snapshot with `<volume_name>/<snapshot_name>`.

To show information about a snapshot, use the following command:

    lxc storage volume show <pool_name> <volume_name>/<snapshot_name>

To edit a snapshot (for example, to add a description or change the expiry date), use the following command:

    lxc storage volume edit <pool_name> <volume_name>/<snapshot_name>

To delete a snapshot, use the following command:

    lxc storage volume delete <pool_name> <volume_name>/<snapshot_name>

````
```` {group-tab} UI

To view, edit or delete a storage volume snapshot, navigate to the {guilabel}`Snapshots` tab for the target volume.

Hover over a snapshot row to highlight possible actions, including {guilabel}`edit`, {guilabel}`restore` and {guilabel}`delete`.

```{figure} /images/storage/storage_volumes_snapshots_list.png
:width: 80%
:alt: LXD Storage Volumes - Snapshots list

````
`````

### Schedule snapshots of a custom storage volume

`````{tabs}
````{group-tab} CLI

You can configure a custom storage volume to automatically create snapshots at specific times.
To do so, set the `snapshots.schedule` configuration option for the storage volume (see {ref}`storage-configure-volume`).

For example, to configure daily snapshots, use the following command:

    lxc storage volume set <pool_name> <volume_name> snapshots.schedule @daily

To configure taking a snapshot every day at 6 am, use the following command:

    lxc storage volume set <pool_name> <volume_name> snapshots.schedule "0 6 * * *"

When scheduling regular snapshots, consider setting an automatic expiry (`snapshots.expiry`) and a naming pattern for snapshots (`snapshots.pattern`).
See the {ref}`storage-drivers` documentation for more information about those configuration options.

````
```` {group-tab} UI

To schedule a snapshot of a storage volume, navigate to the {guilabel}`Overview` tab of the target volume. Select the {guilabel}`Snapshots` tab and click {guilabel}`See configuration`.

```{figure} /images/storage/storage_volumes_snapshots_configuration.png
:width: 80%
:alt: LXD Storage Volumes - Snapshots list

````

In the resulting modal, you can override the default schedule for automatic volume snapshots. Select the time frame via the [Cron expression syntax](https://en.wikipedia.org/wiki/Cron#Cron_expression) or a time interval.

`````

### Restore a snapshot of a custom storage volume

`````{tabs}
````{group-tab} CLI

You can restore a custom storage volume to the state of any of its snapshots.

To do so, you must first stop all instances that use the storage volume.
Then use the following command:

    lxc storage volume restore <pool_name> <volume_name> <snapshot_name>

You can also restore a snapshot into a new custom storage volume, either in the same storage pool or in a different one (even a remote storage pool).
To do so, use the following command:

    lxc storage volume copy <source_pool_name>/<source_volume_name>/<source_snapshot_name> <target_pool_name>/<target_volume_name>

````
```` {group-tab} UI

To restore a storage volume, navigate to the {guilabel}`Snapshots` tab for the target volume, then hover over a snapshot row to view possible actions. Click the {guilabel}`restore` button.

````
`````

(storage-backup-export)=
## Use export files for volume backup

You can export the full content of your custom storage volume to a standalone file that can be stored at any location.
For highest reliability, store the backup file on a different file system to ensure that it does not get lost or corrupted.

### Export a custom storage volume

`````{tabs}
````{group-tab} CLI

Use the following command to export a custom storage volume to a compressed file (for example, `/path/to/my-backup.tgz`):

    lxc storage volume export <pool_name> <volume_name> [<file_path>]

If you do not specify a file path, the export file is saved as `backup.tar.gz` in the working directory.

```{warning}
If the output file already exists, the command overwrites the existing file without warning.
```

<!-- Include start export info -->
You can add any of the following flags to the command:

`--compression`
: By default, the output file uses `gzip` compression.
  You can specify a different compression algorithm (for example, `bzip2`) or turn off compression with `--compression=none`.

`--optimized-storage`
: If your storage pool uses the `btrfs` or the `zfs` driver, add the `--optimized-storage` flag to store the data as a driver-specific binary blob instead of an archive of individual files.
  In this case, the export file can only be used with pools that use the same storage driver.

  Exporting a volume in optimized mode is usually quicker than exporting the individual files.
  Snapshots are exported as differences from the main volume, which decreases their size (quota) and makes them easily accessible.

`--export-version`
: If you intend to import the backup to an older version of LXD, set the version to `1` which will use the original (old) backup metadata format.
Backups using the old format can always be imported on newer versions of LXD.
If the flag is not specified and the server has support for the `backup_metadata_version` API extension, version `2` is used by default.
<!-- Include end export info -->

`--volume-only`
: By default, the export file contains all snapshots of the storage volume.
  Add this flag to export the volume without its snapshots.

````
```` {group-tab} UI

To export a storage volume, navigate to the {guilabel}`Overview` tab for the target volume and select the {guilabel}`Export` button.

In the resulting modal, configure the export file for the storage volume, including its compression mode and expiration.

```{figure} /images/storage/storage_volumes_snapshots_export.png
:width: 60%
:alt: LXD Storage Volumes - Snapshots tab
```

````
`````

### Restore a custom storage volume from an export file

`````{tabs}
````{group-tab} CLI

You can import an export file (for example, `/path/to/my-backup.tgz`) as a new custom storage volume.
To do so, use the following command:

    lxc storage volume import <pool_name> <file_path> [<volume_name>]

If you do not specify a volume name, the original name of the exported storage volume is used for the new volume.
If a volume with that name already (or still) exists in the specified storage pool, the command returns an error.
In that case, either delete the existing volume before importing the backup or specify a different volume name for the import.

````
```` {group-tab} UI

To restore a storage volume from an export file, select {guilabel}`Volumes` from the main navigation, then click the {guilabel}`Create volume` button.

Choose the volume file to upload, and select the storage pool for the volume to be created using the export file.

```{figure} /images/storage/storage_volumes_snapshots_import.png
:width: 60%
:alt: LXD Storage Volumes - Snapshots tab
```

```{admonition} For clustered environments only
:class: note
Within a clustered environment, if a cluster-member-specific storage pool is selected, you can also configure a target cluster member for the volume.
```
````
`````
