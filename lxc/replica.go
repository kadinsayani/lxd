package main

import (
	"errors"

	"github.com/spf13/cobra"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/i18n"
)

type cmdReplica struct {
	global *cmdGlobal
}

func (c *cmdReplica) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("replica")
	cmd.Short = i18n.G("Manage replicas")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Manage cluster links`))

	// Create.
	replicaCreate := cmdReplicaCreate{global: c.global}
	cmd.AddCommand(replicaCreate.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }
	return cmd
}

// Add.
type cmdReplicaCreate struct {
	global *cmdGlobal

	flagTargetCluster string
	flagConfig        []string
	flagDescription   string
}

func (c *cmdReplicaCreate) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("create", i18n.G("<name> --target-cluster <cluster_link> [--description <description>]"))
	cmd.Short = i18n.G("Create replica job")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Create replica job
`))
	cmd.Example = cli.FormatSection("", i18n.G(``))
	cmd.Flags().StringVarP(&c.flagTargetCluster, "target-cluster", "t", "", "Target cluster link")
	cmd.Flags().StringArrayVarP(&c.flagConfig, "config", "c", nil, i18n.G("Config key/value to apply to the new replica job")+"``")
	cmd.Flags().StringVarP(&c.flagDescription, "description", "d", "", "Replica job description")

	cmd.RunE = c.run

	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return c.global.cmpRemotes(toComplete, ":", true, instanceServerRemoteCompletionFilters(*c.global.conf)...)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return cmd
}

func (c *cmdReplicaCreate) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	if c.flagTargetCluster == "" {
		return errors.New("Target cluster must be specified")
	}

	// Parse remote.
	remoteName, replicaName, err := c.global.conf.ParseRemote(args[0])
	if err != nil {
		return err
	}

	_, wrapper := newLocationHeaderTransportWrapper()
	client, err := c.global.conf.GetInstanceServerWithConnectionArgs(remoteName, &lxd.ConnectionArgs{TransportWrapper: wrapper})
	if err != nil {
		return err
	}

	replica := api.ReplicaPost{
		Name:          replicaName,
		TargetCluster: c.flagTargetCluster,
	}

	if c.flagDescription != "" {
		replica.Description = c.flagDescription
	}

	err = client.CreateReplica(replica)
	if err != nil {
		return err
	}

	return nil
}
