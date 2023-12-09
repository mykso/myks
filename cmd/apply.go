package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func init() {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply k8s manifests to current context",
		Long:  `TODO`,
		Annotations: map[string]string{
			ANNOTATION_SMART_MODE: ANNOTATION_TRUE,
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Applying k8s manifests")
			g := myks.New(".")

			if err := g.ValidateRootDir(); err != nil {
				log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
			}

			if err := g.Init(asyncLevel, envAppMap); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
			}

			if err := g.Apply(asyncLevel); err != nil {
				log.Fatal().Err(err).Msg("Unable to apply k8s manifests")
			}
		},
	}

	rootCmd.AddCommand(cmd)
}
