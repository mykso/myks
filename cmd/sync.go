package cmd

import (
	"kwhoosh/internal/kwhoosh"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync vendir configs",
		Long:  "Sync vendir configs",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Syncing vendir configs")
			err := kwhoosh.New(".").Init(nil, nil)
			if err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize kwhoosh")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
