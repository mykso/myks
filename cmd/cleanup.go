package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

const cleanupCmdLongHelp = `Cleanup obsolete manifests and cache entries.

This command will cleanup rendered manifests and cache entries that are no longer needed.
By default, it will cleanup both rendered manifests and cache entries, but you can control
what to cleanup with the flags --manifests and --cache.

Examples:
    # Cleanup all rendered manifests and cache entries
    myks cleanup

    # Cleanup only rendered manifests
    myks cleanup --manifests

    # List cache entries that would be cleaned up
    myks cleanup --cache --dry-run
`

func newCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup obsolete manifests and cache entries",
		Long:  cleanupCmdLongHelp,
		Run: func(cmd *cobra.Command, args []string) {
			dryRun, _ := readFlagBool(cmd, "dry-run")

			if dryRun {
				log.Info().Msg("Running in dry-run mode")
			}

			modeManifests, modeManifestsSet := readFlagBool(cmd, "manifests")
			modeCache, modeCacheSet := readFlagBool(cmd, "cache")

			if !modeManifestsSet && !modeCacheSet {
				modeManifests = true
				modeCache = true
			}

			if !modeManifests && !modeCache {
				log.Fatal().Msg("Nothing to cleanup")
			}

			g := myks.New(".")

			if err := g.ValidateRootDir(); err != nil {
				log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
			}

			if err := g.Init(asyncLevel, envAppMap); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
			}

			if modeManifests {
				if err := g.CleanupRenderedManifests(dryRun); err != nil {
					log.Fatal().Err(err).Msg("Unable to cleanup rendered manifests")
				}
			}

			if modeCache {
				if err := g.CleanupObsoleteCacheEntries(dryRun); err != nil {
					log.Fatal().Err(err).Msg("Unable to cleanup cache entries")
				}
			}
		},
	}

	cmd.Flags().Bool("dry-run", false, "print what would be cleaned up without actually cleaning up")
	cmd.Flags().Bool("manifests", false, "cleanup rendered manifests")
	cmd.Flags().Bool("cache", false, "cleanup cache entries")

	return cmd
}
