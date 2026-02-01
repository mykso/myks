package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mykso/myks/internal/myks"
)

const pluginPrefix = "myks-"

// findPlugins searches for plugins in the PATH and in configured plugin-sources.
// Executables in PATH must have the prefix defined in pluginPrefix.
// Executables in plugin-sources can have any name.
func findPlugins() []myks.Plugin {
	path := filepath.SplitList(os.Getenv("PATH"))
	pluginsPath := myks.FindPluginsInPaths(path, pluginPrefix)

	sources := viper.GetStringSlice("plugin-sources")
	for i, source := range sources {
		sources[i] = os.ExpandEnv(source)
	}
	pluginsLocal := myks.FindPluginsInPaths(sources, "")

	return append(pluginsPath, pluginsLocal...)
}

func addPlugins(cmd *cobra.Command) {
	plugins := findPlugins()

	uniquePlugins := make(map[string]myks.Plugin)
	for _, plugin := range plugins {
		if existing, exists := uniquePlugins[plugin.Name()]; exists {
			log.Warn().
				Str("plugin", plugin.Name()).
				Interface("kept_plugin", existing).
				Interface("skipped_plugin", plugin).
				Msg("Duplicate plugin name detected; skipping plugin with duplicate name")
			continue
		}
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
	cmd := &cobra.Command{
		Use:     plugin.Name() + " [environments [applications]] [flags] [-- <plugin-args>...]",
		Short:   "Execute " + plugin.Name() + " plugin",
		Long:    "Execute the " + plugin.Name() + " plugin for specified environments and applications.",
		GroupID: "Plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			splitAt := cmd.ArgsLenAtDash()
			if splitAt == -1 {
				splitAt = len(args)
			}
			myksArgs, pluginArgs := args[:splitAt], args[splitAt:]

			if len(myksArgs) > 2 {
				return fmt.Errorf("expected at most 2 positional arguments (environments and applications), got %d", len(myksArgs))
			}

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

			bufferOutput := viper.GetBool("buffer-plugin-output")
			if err := g.ExecPlugin(asyncLevel, plugin, pluginArgs, bufferOutput); err != nil {
				log.Fatal().Err(err).Msg("Plugin did not run successfully")
				return err
			}

			return nil
		},
		ValidArgsFunction: shellCompletion,
	}

	cmd.SetUsageTemplate(pluginUsageTemplate(plugin.Name()))

	return cmd
}

func pluginUsageTemplate(pluginName string) string {
	return `Usage:
  {{.CommandPath}} [environments [applications]] [flags] [-- <plugin-args>...]

Arguments:
  environments    (Optional) Comma-separated list of environments or ALL
                  ALL will process all environments
                  Examples: ALL, prod,stage,dev, prod/region1

  applications    (Optional) Comma-separated list of applications or ALL
                  ALL will process all applications
                  Examples: ALL, app1,app2

  -- <plugin-args>
                  (Optional) Arguments to pass to the plugin executable
                  Everything after -- is passed directly to the plugin

{{if .HasAvailableFlags}}Flags:
{{.Flags.FlagUsages | trimTrailingWhitespaces}}{{end}}

Environment Variables:
  The plugin receives the following environment variables:
    MYKS_ENV              - Environment ID
    MYKS_APP              - Application name
    MYKS_APP_PROTOTYPE    - Application prototype name
    MYKS_ENV_DIR          - Environment directory path
    MYKS_RENDERED_APP_DIR - Rendered application directory path
    MYKS_DATA_VALUES      - YAML data values for the application

Examples:
  # Run ` + pluginName + ` for all environments and applications
  {{.CommandPath}} ALL ALL

  # Run ` + pluginName + ` for specific environments
  {{.CommandPath}} prod,stage ALL

  # Run ` + pluginName + ` with arguments passed to the plugin
  {{.CommandPath}} ALL ALL -- --verbose --output json

  # Run ` + pluginName + ` for specific app with plugin arguments
  {{.CommandPath}} prod myapp -- --dry-run
`
}
