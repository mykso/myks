package cmd

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mykso/myks/internal/myks"
)

const pluginPrefix = "myks-"

// findPlugins searches for plugins in the PATH and in configured plugin-sources
func findPlugins() []myks.Plugin {
	path := filepath.SplitList(os.Getenv("PATH"))
	pluginsPath := myks.FindPluginsInPaths(path, pluginPrefix)

	sources := viper.GetStringSlice("plugin-sources")
	for i, source := range sources {
		sources[i] = os.ExpandEnv(source) // supports $HOME but not ~
	}
	pluginsLocal := myks.FindPluginsInPaths(sources, "")

	return append(pluginsPath, pluginsLocal...)
}

func addPlugins(cmd *cobra.Command) {
	plugins := findPlugins()

	uniquePlugins := make(map[string]myks.Plugin)
	for _, plugin := range plugins {
		uniquePlugins[plugin.Name()] = plugin
	}

	if len(uniquePlugins) > 0 {
		cmd.AddGroup(&cobra.Group{ID: "Plugins", Title: "Plugin Subcommands:"})
	}

	for _, plugin := range uniquePlugins {
		cmd.AddCommand(newPluginCmd(plugin))
	}
}

func newPluginCmd(plugin myks.Plugin) *cobra.Command {
	return &cobra.Command{
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
			g := getGlobe()

			if err := g.ValidateRootDir(); err != nil {
				log.Fatal().Err(err).Msg("Root directory is not suitable for myks")
				return err
			}

			if err := g.Init(asyncLevel, envAppMap); err != nil {
				log.Fatal().Err(err).Msg("Unable to initialize globe")
				return err
			}

			if err := g.ExecPlugin(asyncLevel, plugin, pluginArgs); err != nil {
				log.Fatal().Err(err).Msg("Plugin did not run successfully")
				return err
			}

			return nil
		},
		ValidArgsFunction: shellCompletion,
	}
}
