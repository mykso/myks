package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func init() {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize new myks project",
		Long:  "Initialize new myks project",
		Run: func(cmd *cobra.Command, args []string) {
			g := myks.New(".")

			if err := g.Bootstrap(); err != nil {
				log.Fatal().Err(err).Msg("Failed to initialize project")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
