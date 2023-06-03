package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func init() {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Run sync and render",
		Long:  "Run sync and render",
		Run: func(cmd *cobra.Command, args []string) {
			g := myks.New(".")

			if err := g.Init(targetEnvironments, targetApplications); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
			}

			log.Info().Msg("Syncing vendir configs")
			if err := g.Sync(); err != nil {
				log.Fatal().Err(err).Msg("Unable to sync vendir configs")
			}

			log.Info().Msg("Rendering manifests")
			if err := g.Render(); err != nil {
				log.Fatal().Err(err).Msg("Unable to render applications")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
