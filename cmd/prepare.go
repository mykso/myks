package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"kwhoosh/internal/kwhoosh"
)

func init() {
	prepareCmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare vendir configs for syncing",
		Long:  "Prepare vendir configs for syncing",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Preparing vendir configs for syncing")
			err := kwhoosh.New(".").Init(nil, nil)
			if err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize kwhoosh")
			}
		},
	}

	rootCmd.AddCommand(prepareCmd)
}
