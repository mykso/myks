package cmd

import (
	"errors"

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
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to read flag")
			}
			if err := g.Bootstrap(force); errors.Is(err, myks.ErrNotClean) {
				log.Error().Msg("Directory not empty. Use --force to overwrite data.")
			} else if err != nil {
				log.Fatal().Err(err).Msg("Failed to initialize project")
			}
		},
	}
	cmd.Flags().BoolP("force", "f", false, "overwrite existing data")
	rootCmd.AddCommand(cmd)
}
