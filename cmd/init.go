package cmd

import (
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func newInitCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize new myks project",
		Long: `Initialize a new myks project in the current directory.

Creates .myks.yaml, .gitignore, envs/, prototypes/ and rendered/envs/, with two
sample environments (dev and prod) that run one sample Helm chart.
With --argocd, the project also generates Argo CD resources and installs Argo CD in dev.
With --ci github, it also gets GitHub Actions workflows that render pull requests
and check the default branch for stale output. In an existing project, print them
with: myks init --print --components ci-github`,
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

			argocd, err := cmd.Flags().GetBool("argocd")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to read flag")
			}
			if argocd {
				components = append(components, "argocd")
			}

			switch ci, _ := cmd.Flags().GetString("ci"); ci {
			case "":
			case "github":
				components = append(components, "ci-github")
			default:
				log.Fatal().Str("ci", ci).Msg("Unsupported CI system, use: github")
			}

			if err := myks.NewWithDefaults().Bootstrap(force, onlyPrint, components, version); errors.As(err, &myks.ErrBootstrapTargetExists{}) {
				log.Error().Err(err).Msg("The target already exists. Use --force to overwrite data.")
			} else if err != nil {
				log.Fatal().Err(err).Msg("Failed to initialize project")
			}
		},
	}

	cmd.Flags().BoolP("force", "f", false, "overwrite existing data")
	cmd.Flags().String("ci", "", "add CI workflows for the given system (github)")
	cmd.Flags().Bool("argocd", false, "add an Argo CD installation and generate Argo CD resources for the sample environments")

	printHelp := "print configuration instead of creating files\n" +
		"applicable only to the following components: gitignore, config, schema, ci-github"
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
