package command

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terminal",
		Short: "Cirrus Terminal",
	}

	cmd.AddCommand(newServeCmd())

	return cmd
}
