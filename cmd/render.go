package cmd

import (
	"kwhoosh/internal/kwhoosh"

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

			kwh := kwhoosh.New(".")

			if err := kwh.Init(targetEnvironments, targetApplications); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize kwhoosh")
			}

			if err := kwh.Render(); err != nil {
				log.Fatal().Err(err).Msg("Unable to render applications")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
