package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

var cleanUpCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup obsolete manifests",
	Long:  "Cleanup obsolete manifests",
	Run: func(cmd *cobra.Command, args []string) {
		g := myks.New(".")

		if err := g.ValidateRootDir(); err != nil {
			log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
		}

		if err := g.Init(asyncLevel, envAppMap); err != nil {
			log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
		}

		if err := g.CleanupRenderedManifests(); err != nil {
			log.Fatal().Err(err).Msg("Unable to cleanup")
		}
	},
}
