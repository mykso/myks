package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Download external sources",
	Long: `Download external sources for specified environments and applications.

Authentication against protected repositories is achieved with environment variables prefixed with "VENDIR_SECRET_".
For example, if you reference a secret named "mycreds" in your vendir.yaml, you need to export the variables "VENDIR_SECRET_MYCREDS_USERNAME" and
"VENDIR_SECRET_MYCREDS_PASSWORD" in your environment.`,
	Annotations: map[string]string{
		AnnotationSmartMode: AnnotationTrue,
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Msg("Syncing vendir configs")
		g := myks.New(".")

		if err := g.ValidateRootDir(); err != nil {
			log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
		}

		if err := g.Init(asyncLevel, envAppMap); err != nil {
			log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
		}

		if err := g.Sync(asyncLevel); err != nil {
			log.Fatal().Err(err).Msg("Unable to sync vendir configs")
		}
	},
	ValidArgsFunction: shellCompletion,
}
