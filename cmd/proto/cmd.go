package proto

import (
	"github.com/mykso/myks/internal/prototypes"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "proto",
	Short: "Manage prototypes",
	Long:  `Manage prototypes with myks.`,
}

func init() {
	Cmd.PersistentFlags().StringP("prototype", "p", "", "Name of prototype, may include folder")
	Cmd.RegisterFlagCompletionFunc("prototype", prototypeCompletion)
}

var prototypeCompletion = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	protos, err := prototypes.CollectPrototypes()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return protos, cobra.ShellCompDirectiveNoFileComp
}
