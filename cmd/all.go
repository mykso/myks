package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Run prepare, sync, and render",
		Long:  "Run prepare, sync, and render",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Running prepare, sync, and render")
		},
	}

	rootCmd.AddCommand(cmd)
}
