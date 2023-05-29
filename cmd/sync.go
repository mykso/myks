package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"bruh/internal/bruh"
)

func init() {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync vendir configs",
		Long:  "Sync vendir configs",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Syncing vendir configs")
			g := bruh.New(".")

			if err := g.Init(targetEnvironments, targetApplications); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize bruh's globe")
			}

			if err := g.Sync(); err != nil {
				log.Fatal().Err(err).Msg("Unable to sync vendir configs")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
