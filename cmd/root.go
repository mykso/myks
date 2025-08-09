// Package cmd provides commands for myks, a Kubernetes manifest generator.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gv "github.com/hashicorp/go-version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mykso/myks/cmd/embedded"
	"github.com/mykso/myks/internal/myks"
)

const (
	MyksConfigName = ".myks"
	MyksConfigType = "yaml"
)

var (
	cfgFile    string
	envAppMap  myks.EnvAppMap
	asyncLevel int
)

func NewMyksCmd(version, commit, date string) *cobra.Command {
	cobra.OnInitialize(initLogger)
	cobra.OnInitialize(func() { checkMinVersion(version) })
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	cmd := newRootCmd(version, commit, date)
	cmd.AddCommand(newRenderCmd())
	cmd.AddCommand(newCleanupCmd())
	cmd.AddCommand(newInitCmd(version))
	cmd.AddCommand(newPrintConfigCmd())
	cmd.AddCommand(embedded.EmbeddedCmd("vendir", "Vendir is embedded in myks to manage vendir.yaml files."))
	cmd.AddCommand(embedded.EmbeddedCmd("ytt", "Ytt is embedded in myks to manage yaml files."))
	initConfig()
	addPlugins(cmd)

	return cmd
}

func newRootCmd(version, commit, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "myks",
		Short: "Myks generates Kubernetes manifests",
		// TODO: Launch the documentation website and add a link here
		Long: `
Myks - Kubernetes Manifest Generator

OVERVIEW
  Myks simplifies Kubernetes manifest management through a standardized toolset
  and GitOps-ready conventions.

CORE FEATURES
  • External source management (via vendir)
  • Helm chart rendering
  • YAML templating and validation (via ytt)
  • Idempotent output
  • Automatic ArgoCD resource generation
  • Environment-based configuration inheritance
  • Intelligent change detection

BASIC COMMANDS
  init     Create a new Myks project in the current directory
  render   Download external sources and render manifests for specified environments and applications

GETTING STARTED
  1. Create a new project: myks init
  2. Download dependencies and render manifests: myks render [environments [applications]]

LEARN MORE
  • Use 'myks <command> --help' for detailed information about a command
  • Report issues at https://github.com/example/myks/issues
`,
	}

	rootCmd.Version = fmt.Sprintf(`%s
     commit: %s
     date:   %s`, version, commit, date)

	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "set the logging level")

	asyncHelp := "sets the number of applications to be processed in parallel\nthe default (0) is no limit"
	rootCmd.PersistentFlags().IntVarP(&asyncLevel, "async", "a", 0, asyncHelp)

	smartModeBaseRevisionHelp := "base revision to compare against in Smart Mode\n" +
		"if not provided, only local changes will be considered"
	rootCmd.PersistentFlags().String("smart-mode.base-revision", "", smartModeBaseRevisionHelp)

	smartModeOnlyPrintHelp := "only print the list of environments and applications that would be rendered in Smart Mode"
	rootCmd.PersistentFlags().Bool("smart-mode.only-print", false, smartModeOnlyPrintHelp)

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.Fatal().Err(err).Msg("Unable to bind flags")
	}
	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		log.Fatal().Err(err).Msg("Unable to bind flags")
	}

	configHelp := fmt.Sprintf("config file (default is the first %s.%s up the directory tree)", MyksConfigName, MyksConfigType)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", configHelp)

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Annotations[AnnotationSmartMode] != AnnotationTrue {
			return nil
		}
		return initTargetEnvsAndApps(cmd, args)
	}
	return rootCmd
}

func initConfig() {
	viper.SetEnvPrefix("MYKS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName(MyksConfigName)
		viper.SetConfigType(MyksConfigType)

		// Add all parent directories to the config search path
		dir, _ := os.Getwd()
		for {
			dir = filepath.Dir(dir)
			viper.AddConfigPath(dir)
			if dir == "/" || dir == filepath.Dir(dir) {
				break
			}
		}
	}

	err := viper.ReadInConfig()

	if err == nil {
		// TODO: Make paths in config file relative to the config file
		log.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())
	} else {
		log.Debug().Err(err).Msg("Unable to read config file")
	}
}

func checkMinVersion(current string) {
	minVersion := viper.GetString("min-version")
	if minVersion == "" {
		log.Debug().Msg("No min-version specified in config file, skipping check")
		return
	}
	v1, err := gv.NewVersion(minVersion)
	if err != nil {
		log.Error().Err(err).Str("min-version", minVersion).Msg("Invalid min-version specified in config file")
	}
	v2, err := gv.NewVersion(current)
	if err != nil {
		log.Info().Err(err).Str("current-version", current).Msg("Invalid current version, skipping min-version check")
		return
	}
	if v1.GreaterThan(v2) {
		log.Error().Str("min-version", minVersion).Str("current-version", current).Msg("Current version is lower than min-version")
	}
}

func initLogger() {
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.DurationFieldInteger = true

	logLevel, err := zerolog.ParseLevel(viper.GetString("log-level"))
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to parse log level")
	}

	log.Info().Msgf("Setting log level to: %s", logLevel)
	zerolog.SetGlobalLevel(logLevel)
}
