// Package cmd provides commands for myks, a Kubernetes manifest generator.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gv "github.com/hashicorp/go-version"
	aurora "github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/mykso/myks/cmd/embedded"
	"github.com/mykso/myks/internal/myks"
)

// MyksConfigName is the base name for myks configuration files.
const (
	MyksConfigName = ".myks"
	MyksConfigType = "yaml"
)

var (
	cfgFile   string
	envAppMap myks.EnvAppMap
	// TODO: change to uint
	asyncLevel int

	globe *myks.Globe
)

// NewMyksCmd creates the root cobra command for the myks CLI.
func NewMyksCmd(version, commit, date string) *cobra.Command {
	cobra.OnInitialize(initColors)
	cobra.OnInitialize(initLogger)
	cobra.OnInitialize(func() { checkMinVersion(version) })
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	cmd := newRootCmd(version, commit, date)
	cmd.AddCommand(newRenderCmd())
	cmd.AddCommand(newCleanupCmd())
	cmd.AddCommand(newInitCmd(version))
	cmd.AddCommand(newPrintConfigCmd())
	cmd.AddCommand(newInspectCmd())
	cmd.AddCommand(embedded.Cmd("vendir", "Vendir is embedded in myks to manage vendir.yaml files."))
	cmd.AddCommand(embedded.Cmd("ytt", "Ytt is embedded in myks to manage yaml files."))
	cmd.AddCommand(embedded.Cmd("kbld", "Kbld is embedded in myks to manage container image references."))
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
  • Report issues at https://github.com/mykso/myks/issues
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

	smartModeOnlyPrintHelp := "only print the list of environments and applications that would be rendered in Smart Mode\n" +
		"accepted values: \"text\" (default), \"json\"\n" +
		"legacy boolean values (true, 1, yes) are treated as \"text\" for backward compatibility\n" +
		"legacy falsey values (false, 0, no) disable Smart Mode only-print (same as leaving it empty)\n" +
		"when used without a value (--smart-mode.only-print), defaults to \"text\""
	rootCmd.PersistentFlags().String("smart-mode.only-print", "", smartModeOnlyPrintHelp)
	rootCmd.PersistentFlags().Lookup("smart-mode.only-print").NoOptDefVal = outputFormatText

	bufferPluginOutputHelp := "buffer plugin output instead of streaming (useful for parallel execution)"
	rootCmd.PersistentFlags().Bool("buffer-plugin-output", false, bufferPluginOutputHelp)

	rootCmd.PersistentFlags().Bool("print-stats", false, "print tool resource statistics directly to stdout")

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

	viper.SetDefault("root-dir", ".")
	if err := viper.ReadInConfig(); err == nil {
		log.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())
		if viper.GetBool("config-in-root") {
			rootDir := filepath.Dir(viper.ConfigFileUsed())
			viper.Set("root-dir", rootDir)
			log.Info().Msgf("Setting root-dir to: %s", rootDir)
		}
	} else if notFound := (viper.ConfigFileNotFoundError{}); errors.As(err, &notFound) {
		log.Debug().Msg("Config file not found, using defaults")
	} else {
		log.Error().Err(err).Msg("Error reading config file")
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

func initColors() {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor || !term.IsTerminal(int(os.Stdout.Fd())) { // #nosec G115 -- file descriptor fits in int
		aurora.DefaultColorizer = aurora.New(aurora.WithColors(false))
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

	myks.PrintStats = viper.GetBool("print-stats")
}
