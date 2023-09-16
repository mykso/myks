package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func init() {
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render manifests",
		Long:  "Render manifests",
		Annotations: map[string]string{
			ANNOTATION_SMART_MODE: ANNOTATION_TRUE,
		},
		Run: func(cmd *cobra.Command, args []string) {
			g := myks.New(".")

			if err := g.Init(asyncLevel, targetEnvironments, targetApplications); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize globe")
			}

			if err := g.Render(asyncLevel); err != nil {
				log.Fatal().Err(err).Msg("Unable to render applications")
			}

			if targetEnvironments == nil {
				if err := g.Cleanup(); err != nil {
					log.Fatal().Err(err).Msg("Unable to cleanup")
				}
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
