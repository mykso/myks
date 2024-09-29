package proto

import (
	"os"

	"github.com/mykso/myks/internal/myks"
	"github.com/mykso/myks/internal/prototypes"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	Cmd.AddCommand(newProtoDelCmd())
}

func newProtoDelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete",
		Short:             "Delete prototype",
		Long:              `Delete a prototype`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: prototypeCompletion,
		Run: func(cmd *cobra.Command, args []string) {
			// start
			g := myks.New(".")
			for _, prototype := range args {
				err := prototypes.Delete(g, prototype)
				if err != nil {
					if !os.IsNotExist(err) {
						log.Err(err).Str("prototype", prototype).Msg("Prototype was not deleted")
						cobra.CheckErr(err)
					}
					log.Warn().Str("prototype", prototype).Msg("Prototype not found")
					return
				}
				log.Info().Str("prototype", prototype).Msg("Prototype deleted")
			}
		},
	}

	return cmd
}
