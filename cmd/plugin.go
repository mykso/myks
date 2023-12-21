package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func init() {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin commands",
		Long:  "Provides commands for plugin management in myks",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().Msg("Listing plugins")
			for _, p := range listPlugins() {
				log.Info().Msg(p.Name())
			}
		},
	}
	rootCmd.AddGroup(&cobra.Group{ID: "Plugins", Title: "Plugin Subcommands:"})
	addPlugins(rootCmd)
	rootCmd.AddCommand(cmd)
}

func listPlugins() []myks.Plugin {
	plugins := myks.FindPluginsInPaths(nil)
	localPlugins := myks.FindPluginsInPaths([]string{"plugins"})
	return append(plugins, localPlugins...)
}

func addPlugins(cmd *cobra.Command) {
	plugins := listPlugins()
	for _, plugin := range plugins {
		func(plugin myks.Plugin) {
			cmd.AddCommand(&cobra.Command{
				Use:     plugin.Name(),
				Short:   "Execute " + plugin.Name(),
				Long:    "Execute" + plugin.Name(),
				GroupID: "Plugins",
				Annotations: map[string]string{
					ANNOTATION_SMART_MODE: ANNOTATION_TRUE,
				},
				Run: func(cmd *cobra.Command, args []string) {
					g := myks.New(".")

					if err := g.ValidateRootDir(); err != nil {
						log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
					}

					if err := g.Init(asyncLevel, envAppMap); err != nil {
						log.Fatal().Err(err).Msg("Unable to initialize globe")
					}

					if err := g.ExecPlugin(asyncLevel, plugin); err != nil {
						log.Fatal().Err(err).Msg("Unable to render applications")
					}

					// Cleaning up only if all environments and applications were processed
					if envAppMap == nil {
						if err := g.Cleanup(); err != nil {
							log.Fatal().Err(err).Msg("Unable to cleanup")
						}
					}
				},
			})
		}(plugin)
	}
}
