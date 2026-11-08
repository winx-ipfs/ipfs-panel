package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/auth"
	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/node"
)

// resetAdmin deletes the admin credential file. It does not take a password:
// a password on the command line is visible in the process list and in shell
// history. Stop the manager first so the running process drops its sessions.
func resetAdmin(args []string) error {
	dataDefault := os.Getenv("IPFS_PANEL_DATA")
	if dataDefault == "" {
		var err error
		dataDefault, err = node.DefaultDataDir()
		if err != nil {
			return err
		}
	}
	repoDefault := os.Getenv("IPFS_PANEL_REPO")
	fs := flag.NewFlagSet("reset-admin", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	data := fs.String("data", dataDefault, "manager data directory (env IPFS_PANEL_DATA)")
	repo := fs.String("repo", repoDefault, "panel Kubo repo, used only to refuse a file inside it (env IPFS_PANEL_REPO)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", fs.Args())
	}
	path := auth.AuthPath(*data)
	if err := auth.OutsideRepo(path, *repo); err != nil {
		return err
	}
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		_, err = fmt.Fprintln(os.Stdout, "No admin account was stored. Start the manager and create one.")
		return err
	}
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Removed the admin account (%s).\n", path); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, "Stop the manager if it is still running, then start it again."); err != nil {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, "The new process prints a one-time setup token. On this machine, open the panel and create the admin. From another machine, paste that token into the setup screen.")
	return err
}
