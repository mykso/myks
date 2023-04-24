package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"kwhoosh/internal/kwhoosh"
)

func init() {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync vendir configs",
		Long:  "Sync vendir configs",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Syncing vendir configs")
			kwh := kwhoosh.New(".")

			if err := kwh.Init(targetEnvironments, targetApplications); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize kwhoosh")
			}

			if err := kwh.Sync(); err != nil {
				log.Fatal().Err(err).Msg("Unable to sync vendir configs")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
