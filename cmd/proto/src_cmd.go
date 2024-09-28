package proto

import (
	"github.com/spf13/cobra"
)

func init() {
	Cmd.AddCommand(newProtoSrcCmd())
}

func newProtoSrcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "src",
		Short: "Manage prototype sources",
		Long:  `Add a new source to a prototype.`,
	}

	cmd.AddCommand(newProtoAddSrcCmd())
	return cmd
}
