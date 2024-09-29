package proto

import (
	"github.com/mykso/myks/cmd/proto/tui"
	"github.com/mykso/myks/internal/myks"
	"github.com/spf13/cobra"
)

func init() {
	Cmd.AddCommand(newProtoListCmd())
}

func newProtoListCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:    "ui",
		Short:  "Show prototypes in interactive mode",
		Long:   `View and manage prototypes in an interactive mode`,
		Hidden: true,
		// Args:              cobra.ExactArgs(1),
		// ValidArgsFunction: prototypeCompletion,
		Run: func(cmd *cobra.Command, args []string) {
			g := myks.New(".")
			err := tui.New(g)
			cobra.CheckErr(err)
		},
	}
	return cmd
}
