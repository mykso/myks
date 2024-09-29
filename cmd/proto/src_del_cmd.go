package proto

import (
	"os"

	"github.com/mykso/myks/internal/myks"
	"github.com/mykso/myks/internal/prototypes"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newProtoDelSrcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete",
		Short:             "Delete prototype src",
		Long:              `Delete a source prototype from a prototype.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: prototypeCompletion,

		Run: func(cmd *cobra.Command, args []string) {
			name, err := cmd.Flags().GetString("name")
			cobra.CheckErr(err)
			if name == "" {
				cobra.CheckErr("Name must be provided")
			}

			// start
			g := myks.New(".")

			p, err := prototypes.Load(g, prototype)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Err(err).Str("prototype", prototype).Msg("Invalid prototype file")
					os.Exit(1)
				}
				log.Error().Str("prototype", prototype).Msg("Prototype not found")
				os.Exit(1)
			}

			if _, exist := p.GetSource(name); !exist {
				log.Warn().Str("prototype", prototype).Str("source", name).Msg("Source does not exist")
				return
			}
			p.DelSource(name)
			err = p.Save()
			cobra.CheckErr(err)
			log.Info().Str("prototype", prototype).Str("source", name).Msg("Prototype source removed")
		},
	}

	cmd.Flags().StringP("name", "n", "", "Name of prototype, may include folder")
	cobra.CheckErr(cmd.MarkFlagRequired("name"))

	return cmd
}
