package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/canonical/lxd/shared/i18n"
	"github.com/canonical/lxd/shared/termios"
)

type cmdReplica struct {
	global *cmdGlobal
}

func (c *cmdReplica) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("replica")
	cmd.Short = i18n.G("Manage replicas")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Manage replicas`))

	// Create.
	replicaCreate := cmdReplicaCreate{global: c.global}
	cmd.AddCommand(replicaCreate.command())

	// Run.
	replicaRun := cmdReplicaRun{global: c.global}
	cmd.AddCommand(replicaRun.command())

	// Delete.
	replicaDeleteCmd := cmdReplicaDelete{global: c.global}
	cmd.AddCommand(replicaDeleteCmd.command())

	// List.
	replicaListCmd := cmdReplicaList{global: c.global}
	cmd.AddCommand(replicaListCmd.command())

	// Workaround for subcommand usage errors. See: https://github.com/spf13/cobra/issues/706
	cmd.Args = cobra.NoArgs
	cmd.Run = func(cmd *cobra.Command, args []string) { _ = cmd.Usage() }
	return cmd
}

// Add.
type cmdReplicaCreate struct {
	global *cmdGlobal

	flagDescription string
}

func (c *cmdReplicaCreate) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("create", i18n.G("[<remote>:]<replica> [key=value...]"))
	cmd.Short = i18n.G("Create replica")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Create replica
`))
	cmd.Example = cli.FormatSection("", i18n.G(``))
	cmd.Flags().StringVarP(&c.flagDescription, "description", "d", "", "Replica description")

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
	var stdinData api.ReplicaPut

	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, -1)
	if exit {
		return err
	}

	// If stdin isn't a terminal, read text from it
	if !termios.IsTerminal(getStdinFd()) {
		contents, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

		err = yaml.Unmarshal(contents, &stdinData)
		if err != nil {
			return err
		}
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
		Name:       replicaName,
		Project:    c.global.flagProject,
		ReplicaPut: stdinData,
	}

	if c.flagDescription != "" {
		replica.Description = c.flagDescription
	}

	if stdinData.Config == nil {
		replica.Config = map[string]string{}
		for i := 1; i < len(args); i++ {
			entry := strings.SplitN(args[i], "=", 2)
			if len(entry) < 2 {
				return fmt.Errorf(i18n.G("Bad key=value pair: %s"), entry)
			}

			replica.Config[entry[0]] = entry[1]
		}
	}

	err = client.CreateReplica(replica)
	if err != nil {
		return err
	}

	return nil
}

// Run.
type cmdReplicaRun struct {
	global *cmdGlobal

	flagRestore bool
}

func (c *cmdReplicaRun) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("run", i18n.G("[<remote>:]<replica>"))
	cmd.Short = i18n.G("Run replica")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Run replica
`))
	cmd.Example = cli.FormatSection("", i18n.G(``))

	cmd.RunE = c.run
	cmd.Flags().BoolVar(&c.flagRestore, "restore", false, i18n.G("Restore instances from a replica target"))

	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return c.global.cmpTopLevelResource("replica", toComplete)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return cmd
}

func (c *cmdReplicaRun) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
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

	replicaPost := api.ReplicaPost{
		Name:    replicaName,
		Restore: c.flagRestore,
	}

	err = client.RunReplica(replicaPost)
	if err != nil {
		return err
	}

	return nil
}

// Delete.
type cmdReplicaDelete struct {
	global *cmdGlobal
}

func (c *cmdReplicaDelete) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("delete", i18n.G("[<remote>:]<replica>"))
	cmd.Aliases = []string{"rm"}
	cmd.Short = i18n.G("Delete replicas")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`Delete replicas`))

	cmd.RunE = c.run

	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return c.global.cmpTopLevelResource("replica", toComplete)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return cmd
}

func (c *cmdReplicaDelete) run(cmd *cobra.Command, args []string) error {
	// Quick checks
	exit, err := c.global.CheckArgs(cmd, args, 1, 1)
	if exit {
		return err
	}

	// Parse remote
	resources, err := c.global.ParseServers(args[0])
	if err != nil {
		return err
	}

	resource := resources[0]
	client := resource.server

	err = client.DeleteReplica(resource.name)
	if err != nil {
		return err
	}

	if !c.global.flagQuiet {
		fmt.Printf(i18n.G("Replica %s deleted")+"\n", resource.name)
	}

	return nil
}

// List.
type cmdReplicaList struct {
	global     *cmdGlobal
	flagFormat string
}

func (c *cmdReplicaList) command() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Use = usage("list", i18n.G("[<remote>:]"))
	cmd.Aliases = []string{"ls"}
	cmd.Short = i18n.G("List replicas")
	cmd.Long = cli.FormatSection(i18n.G("Description"), i18n.G(
		`List replicas`))

	cmd.RunE = c.run
	cmd.Flags().StringVarP(&c.flagFormat, "format", "f", "table", i18n.G("Format (csv|json|table|yaml|compact)")+"``")

	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return c.global.cmpRemotes(toComplete, ":", true, instanceServerRemoteCompletionFilters(*c.global.conf)...)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return cmd
}

func (c *cmdReplicaList) run(cmd *cobra.Command, args []string) error {
	// Quick checks
	exit, err := c.global.CheckArgs(cmd, args, 0, 1)
	if exit {
		return err
	}

	// Parse remote
	remote := ""
	if len(args) > 0 {
		remote = args[0]
	}

	resources, err := c.global.ParseServers(remote)
	if err != nil {
		return err
	}

	resource := resources[0]
	client := resource.server

	replicas, err := client.GetReplicas()
	if err != nil {
		return err
	}

	const layout = "2006/01/02 15:04 MST"

	data := [][]string{}
	for _, replica := range replicas {
		details := []string{
			replica.Name,
			replica.Description,
			replica.Project,
		}

		if shared.TimeIsSet(replica.LastRunAt) {
			details = append(details, replica.LastRunAt.Local().Format(layout))
		}

		data = append(data, details)
	}

	sort.Sort(cli.SortColumnsNaturally(data))

	header := []string{
		i18n.G("NAME"),
		i18n.G("DESCRIPTION"),
		i18n.G("PROJECT"),
		i18n.G("LAST RUN"),
	}

	return cli.RenderTable(c.flagFormat, header, data, replicas)
}
