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
		Run: func(cmd *cobra.Command, args []string) {
			prototype, err := cmd.Flags().GetString("prototype")
			cobra.CheckErr(err)
			if prototype == "" {
				cobra.CheckErr("Prototype must be provided")
			}
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
