package cmd

import (
	"github.com/mykso/myks/internal/myks"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync vendir configs",
		Long: `Sync vendir configs. This will run vendir sync for all applications.

Authentication against protected repositories is achieved with environment variables prefixed with "VENDIR_SECRET_".
For example, if you reference a secret named "mycreds" in your vendir.yaml, you need to export the variables "VENDIR_SECRET_MYCREDS_USERNAME" and
"VENDIR_SECRET_MYCREDS_PASSWORD" in your environment.`,
		Annotations: map[string]string{
			ANNOTATION_SMART_MODE: ANNOTATION_TRUE,
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

	return cmd
}
