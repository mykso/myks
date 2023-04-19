package cmd

import (
	"kwhoosh/internal/kwhoosh"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	prepareCmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare vendir configs for syncing",
		Long:  "Prepare vendir configs for syncing",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Preparing vendir configs for syncing")
			kwhoosh.New(".").CollectEnvironments(nil)
		},
	}

	rootCmd.AddCommand(prepareCmd)
}
