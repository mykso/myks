package cmd

import (
	"github.com/mykso/myks/internal/myks"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Run sync and render",
		Long:  "Run sync and render",
		Run: func(cmd *cobra.Command, args []string) {
			g := myks.New(".")

			if err := g.Init(asyncLevel, targetEnvironments, targetApplications); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
			}

			if err := g.SyncAndRender(asyncLevel); err != nil {
				log.Fatal().Err(err).Msg("Unable to sync vendir configs")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
