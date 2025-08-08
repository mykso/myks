package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Run sync and render",
	Long:  "Run sync and render for specified environments and applications",
	Args:  cobra.RangeArgs(0, 2),
	Annotations: map[string]string{
		AnnotationSmartMode: AnnotationTrue,
	},
	Run: func(cmd *cobra.Command, args []string) {
		RunAllCmd()
	},
	ValidArgsFunction: shellCompletion,
}

func RunAllCmd() {
	g := myks.New(".")

	if err := g.ValidateRootDir(); err != nil {
		log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
	}

	if err := g.Init(asyncLevel, envAppMap); err != nil {
		log.Fatal().Err(err).Msg("Unable to initialize myks' globe")
	}

	if err := g.SyncAndRender(asyncLevel); err != nil {
		log.Fatal().Err(err).Msg("Unable to sync and render applications")
	}

	// Cleaning up only if all environments and applications were processed
	if envAppMap == nil {
		if err := g.CleanupRenderedManifests(false); err != nil {
			log.Fatal().Err(err).Msg("Unable to cleanup rendered manifests")
		}
	}
}
