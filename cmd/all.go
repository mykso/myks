package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

var allCmd = &cobra.Command{
	Use:   "all [environments] [applications]",
	Short: "Run sync and render for selected environments and applications",
	Long: `Run sync and render for selected environments and applications

Usage:
  myks all [environments] [applications] [flags]

Environments:
  A comma-separated list of environments to render. If you provide "ALL", all environments will be rendered.

Applications:
  A comma-separated list of applications to render. If you don't provide this argument or provide "ALL", all applications will be rendered.

Flags:
  -h, --help   help for all

Global Flags:
  -a, --async int                         sets the number of applications to be processed in parallel
                                          the default (0) is no limit
      --config string                     config file (default is the first .myks.yaml up the directory tree)
  -l, --log-level string                  set the logging level (default "info")
      --smart-mode.base-revision string   base revision to compare against in Smart Mode
                                          if not provided, only local changes will be considered
      --smart-mode.only-print             only print the list of environments and applications that would be rendered in Smart Mode`,
	Annotations: map[string]string{
		ANNOTATION_SMART_MODE: ANNOTATION_TRUE,
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
