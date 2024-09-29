package proto

import (
	"fmt"

	"github.com/mykso/myks/internal/prototypes"
	"github.com/spf13/cobra"
)

var prototype string
var Cmd = &cobra.Command{
	Use:   "proto",
	Short: "Manage prototypes",
	Long:  `Manage prototypes with myks.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		proto := args[0]
		if proto == "" {
			return fmt.Errorf("prototype must be provided")
		}
		prototype = proto
		return nil
	},
}

func init() {

	Cmd.RegisterFlagCompletionFunc("prototype", prototypeCompletion)
}

var prototypeCompletion = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if cmd.Args(cmd, append(args, toComplete)) != nil {
		// Do not return prototype completion if there no more args are expected
		// This is especially useful for all commands which are expecting a single prototype
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	protos, err := prototypes.CollectPrototypes()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	//remove already provided prototypes. This is useful for commands which are expecting multiple prototypes (e.g. delete)
	for _, arg := range args {
		for i, proto := range protos {
			if proto == arg {
				protos = append(protos[:i], protos[i+1:]...)
			}
		}
	}
	return protos, cobra.ShellCompDirectiveNoFileComp

}
