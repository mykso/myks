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

	cmd.AddCommand(newProtoModSrcCmd(true))
	cmd.AddCommand(newProtoModSrcCmd(false))
	cmd.AddCommand(newProtoDelSrcCmd())
	cmd.AddCommand(newProtoBumpCmd())
	return cmd
}
