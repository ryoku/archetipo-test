package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryoku/kubegate/internal/cli"
	"github.com/spf13/cobra"
)

func main() {
	configDir, err := cli.ConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to resolve config directory: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := &cobra.Command{
		Use:           "kubegate",
		Short:         "KubeGate CLI — Kubernetes deployment governance",
		SilenceUsage:  true,
		SilenceErrors: true,
		// All sub-commands require authentication unless they override this.
		PersistentPreRunE: cli.RequireAuthPreRun(configDir),
	}
	root.SetContext(ctx)

	// login and logout are exempt from the auth pre-run check.
	noAuth := func(*cobra.Command, []string) error { return nil }

	loginCmd := cli.NewLoginCmd(configDir)
	loginCmd.PersistentPreRunE = noAuth
	logoutCmd := cli.NewLogoutCmd(configDir)
	logoutCmd.PersistentPreRunE = noAuth

	root.AddCommand(loginCmd, logoutCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
