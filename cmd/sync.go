package cmd

import (
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
		},
	}

	rootCmd.AddCommand(cmd)
}
