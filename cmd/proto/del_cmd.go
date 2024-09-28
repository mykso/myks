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
		Use:   "delete",
		Short: "Delete prototype",
		Long:  `Delete a prototype`,
		Run: func(cmd *cobra.Command, args []string) {
			prototype, err := cmd.Flags().GetString("prototype")
			cobra.CheckErr(err)
			if prototype == "" {
				cobra.CheckErr("Name must be provided")
			}
			// start
			g := myks.New(".")

			err = prototypes.Delete(g, prototype)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Err(err).Str("prototype", prototype).Msg("Prototype was not deleted")
					cobra.CheckErr(err)
				}
				log.Warn().Str("prototype", prototype).Msg("Prototype not found")
				return
			}
			log.Info().Str("prototype", prototype).Msg("Prototype deleted")
		},
	}

	cmd.Flags().StringP("prototype", "p", "", "Name of prototype, may include folder")
	cmd.MarkFlagRequired("prototype")

	return cmd
}
