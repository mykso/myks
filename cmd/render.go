package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render manifests",
		Long:  "Render manifests",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Rendering manifests")
		},
	}

	rootCmd.AddCommand(cmd)
}
