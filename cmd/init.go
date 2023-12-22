package cmd

import (
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize new myks project",
		Long:  "Initialize new myks project",
		Run: func(cmd *cobra.Command, args []string) {
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to read flag")
			}

			onlyPrint, err := cmd.Flags().GetBool("print")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to read flag")
			}

			components, err := cmd.Flags().GetStringSlice("components")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to read flag")
			}

			if err := myks.New(".").Bootstrap(force, onlyPrint, components); errors.As(err, &myks.ErrBootstrapTargetExists{}) {
				log.Error().Err(err).Msg("The target already exists. Use --force to overwrite data.")
			} else if err != nil {
				log.Fatal().Err(err).Msg("Failed to initialize project")
			}
		},
	}

	cmd.Flags().BoolP("force", "f", false, "overwrite existing data")

	printHelp := "print configuration instead of creating files\n" +
		"applicable only to the following components: gitingore, config, schema"
	cmd.Flags().Bool("print", false, printHelp)

	componentsDefault := []string{
		"config",
		"environments",
		"gitignore",
		"prototypes",
		"schema",
	}
	cmd.Flags().StringSlice("components", componentsDefault, "components to initialize")

	return cmd
}
