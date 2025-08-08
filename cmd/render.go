package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render manifests",
	Long:  "Render manifests for specified environments and applications",
	Annotations: map[string]string{
		AnnotationSmartMode: AnnotationTrue,
	},
	Run: func(cmd *cobra.Command, args []string) {
		g := myks.New(".")

		if err := g.ValidateRootDir(); err != nil {
			log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
		}

		if err := g.Init(asyncLevel, envAppMap); err != nil {
			log.Fatal().Err(err).Msg("Unable to initialize globe")
		}

		if err := g.Render(asyncLevel); err != nil {
			log.Fatal().Err(err).Msg("Unable to render applications")
		}

		// Cleaning up only if all environments and applications were processed
		if envAppMap == nil {
			if err := g.CleanupRenderedManifests(false); err != nil {
				log.Fatal().Err(err).Msg("Unable to cleanup rendered manifests")
			}
		}
	},
	ValidArgsFunction: shellCompletion,
}
