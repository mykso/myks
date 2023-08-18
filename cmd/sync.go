package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func init() {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync vendir configs",
		Long: `Sync vendir configs. This will run vendir sync for all applications. 

Authentication against protected repositories is achieved with environment variables prefixed with "VENDIR_SECRET_". 
For example, if you reference a secret named "mycreds" in your vendir.yaml, you need to export the variables "VENDIR_SECRET_MYCREDS_USERNAME" and 
"VENDIR_SECRET_MYCREDS_PASSWORD" in your environment.`,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Syncing vendir configs")
			g := myks.New(".")

			if err := g.Init(asyncLevel, targetEnvironments, targetApplications); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
			}

			if err := g.Sync(asyncLevel); err != nil {
				log.Fatal().Err(err).Msg("Unable to sync vendir configs")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
