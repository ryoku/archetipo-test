package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewLogoutCmd returns the "kubegate logout" command.
func NewLogoutCmd(configDir string) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored authentication token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := DeleteToken(configDir); err != nil {
				return fmt.Errorf("removing token: %w", err)
			}
			_, _ = fmt.Fprintln(os.Stdout, "Logged out.")
			return nil
		},
	}
}
