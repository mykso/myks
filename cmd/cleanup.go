package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
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
				log.Fatal().Msg("Both --manifests and --cache flags are set to false, nothing to cleanup")
			}

			g := getGlobe()

			if err := g.ValidateRootDir(); err != nil {
				log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
			}

			if err := g.InitForCleanup(asyncLevel, envAppMap); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize myks's globe")
			}

			switch {
			case modeManifests && modeCache:
				eg := errgroup.Group{}
				eg.Go(func() error { return g.CleanupRenderedManifests(dryRun) })
				eg.Go(func() error { return g.CleanupObsoleteCacheEntries(asyncLevel, dryRun) })
				if err := eg.Wait(); err != nil {
					log.Fatal().Err(err).Msg("Cleanup failed")
				}
			case modeManifests:
				if err := g.CleanupRenderedManifests(dryRun); err != nil {
					log.Fatal().Err(err).Msg("Unable to cleanup rendered manifests")
				}
			case modeCache:
				if err := g.CleanupObsoleteCacheEntries(asyncLevel, dryRun); err != nil {
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
