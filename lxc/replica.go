package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	lxd "github.com/canonical/lxd/client"
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
