package proto

import (
	"github.com/mykso/myks/internal/myks"
	"github.com/mykso/myks/internal/prototypes"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	Cmd.AddCommand(newProtoAddCmd())
}

func newProtoAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add prototype",
		Long:  `Create a new prototype or extend an existing.`,
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var comps []string
			if len(args) == 0 {
				comps = cobra.AppendActiveHelp(comps, "Prototype name must be provided")
			}
			return comps, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			// start
			g := myks.New(".")

			p, err := prototypes.Create(g, prototype)
			cobra.CheckErr(err)

			err = p.Save()
			cobra.CheckErr(err)
			log.Info().Str("prototype", prototype).Msg("Prototype create")
		},
	}

	return cmd
}
