package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mykso/myks/internal/myks"
)

func listPlugins() []myks.Plugin {
	sources := viper.GetStringSlice("plugin-sources")
	plugins := myks.FindPluginsInPaths(nil)
	localPlugins := myks.FindPluginsInPaths(sources)
	return append(plugins, localPlugins...)
}

func addPlugins(cmd *cobra.Command) {
	plugins := listPlugins()

	uniquePlugins := make(map[string]myks.Plugin)
	for _, plugin := range plugins {
		uniquePlugins[plugin.Name()] = plugin
	}

	if len(uniquePlugins) > 0 {
		cmd.AddGroup(&cobra.Group{ID: "Plugins", Title: "Plugin Subcommands:"})
	}

	for _, plugin := range uniquePlugins {
		func(plugin myks.Plugin) {
			cmd.AddCommand(&cobra.Command{
				Use:     plugin.Name(),
				Short:   "Execute " + plugin.Name(),
				Long:    "Execute" + plugin.Name(),
				GroupID: "Plugins",
				RunE: func(cmd *cobra.Command, args []string) error {
					splitAt := cmd.ArgsLenAtDash()
					if splitAt == -1 {
						splitAt = len(args)
					}
					myksArgs, pluginArgs := args[:splitAt], args[splitAt:]

					if err := initTargetEnvsAndApps(cmd, myksArgs); err != nil {
						return err
					}
					g := myks.New(".")

					if err := g.ValidateRootDir(); err != nil {
						log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
						return err
					}

					if err := g.Init(asyncLevel, envAppMap); err != nil {
						log.Fatal().Err(err).Msg("Unable to initialize globe")
						return err
					}

					if err := g.ExecPlugin(asyncLevel, plugin, pluginArgs); err != nil {
						log.Fatal().Err(err).Msg("Unable to render applications")
						return err
					}

					return nil
				},
			})
		}(plugin)
	}
}
