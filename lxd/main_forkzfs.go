package main

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/canonical/lxd/shared"
)

type cmdForkZFS struct {
	global *cmdGlobal
}

func (c *cmdForkZFS) command() *cobra.Command {
	// Main subcommand
	cmd := &cobra.Command{}
	cmd.Use = "forkzfs [<arguments>...]"
	cmd.Short = "Run ZFS inside a cleaned up mount namespace"
	cmd.Long = `Description:
  Run ZFS inside a cleaned up mount namespace

  This internal command is used to run ZFS in some specific cases.
`
	cmd.RunE = c.run
	cmd.Hidden = true

	return cmd
}

func (c *cmdForkZFS) run(cmd *cobra.Command, args []string) error {
	// Quick checks.
	if len(args) < 1 {
		_ = cmd.Help()

		if len(args) == 0 {
			return nil
		}

		return errors.New("Missing required arguments")
	}

	// Only root should run this
	if os.Geteuid() != 0 {
		return errors.New("This must be run as root")
	}

	// Mark mount tree as private
	err := unix.Mount("none", "/", "", unix.MS_REC|unix.MS_PRIVATE, "")
	if err != nil {
		return err
	}

	// Expand the mount path
	absPath, err := filepath.Abs(shared.VarPath())
	if err != nil {
		return err
	}

	expPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		expPath = absPath
	}

	// Find the source mount of the path
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return err
	}

	defer func() { _ = file.Close() }()

	// Unmount all mounts under LXD directory
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		rows := strings.Fields(line)

		if !strings.HasPrefix(rows[4], expPath) {
			continue
		}

		_ = unix.Unmount(rows[4], unix.MNT_DETACH)
	}

	// Run the ZFS command
	command := exec.Command("zfs", args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err = command.Run()
	if err != nil {
		return err
	}

	return nil
}
