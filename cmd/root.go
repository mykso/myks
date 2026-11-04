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
	cobra.OnInitialize(initAsync)
	cobra.OnInitialize(func() { checkMinVersion(version) })
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	cmd := newRootCmd(version, commit, date)
	cmd.AddCommand(newRenderCmd())
	cmd.AddCommand(newCleanupCmd())
	cmd.AddCommand(newInitCmd(version))
	cmd.AddCommand(newMigrateCmd())
	cmd.AddCommand(newPrintConfigCmd())
	cmd.AddCommand(newInspectCmd())
	cmd.AddCommand(embedded.Cmd("vendir", "Vendir is embedded in myks to manage vendir.yaml files."))
	cmd.AddCommand(embedded.Cmd("ytt", "Ytt is embedded in myks to manage yaml files."))
	cmd.AddCommand(embedded.Cmd("kbld", "Kbld is embedded in myks to manage container image references."))
	cfgFile = configFlagValue(os.Args[1:])
	initConfig()
	addPlugins(cmd)

	return cmd
}

func newRootCmd(version, commit, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "myks",
		Short: "Myks generates Kubernetes manifests",
		Long: `Myks renders the Kubernetes manifests of many clusters from one Git repository.

It downloads Helm charts, Git repositories and plain YAML (vendir), renders them
for each environment with Helm and ytt, optionally pins images by digest (kbld),
and writes plain YAML to rendered/, plus Argo CD Applications when enabled.
Commit that output, and every change is a reviewable diff.

Start with:
  myks init      create a sample project
  myks render    download sources and render manifests (Smart Mode: only what changed)
  myks inspect   show environments and applications, and which file sets each value

Documentation: https://mykso.github.io/myks/
Issues:        https://github.com/mykso/myks/issues`,
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

// configFlagValue returns the value of the last --config flag in args, ignoring arguments after "--".
// Cobra parses flags only in Execute, but the config file has to be read before that: its
// plugin-sources register the plugin subcommands that cobra then parses the command line against.
func configFlagValue(args []string) string {
	value := ""
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--":
			return value
		case arg == "--config" && i+1 < len(args):
			i++
			value = args[i]
		case strings.HasPrefix(arg, "--config="):
			value = strings.TrimPrefix(arg, "--config=")
		}
	}
	return value
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
		if viper.GetBool("config-in-root") {
			viper.Set("root-dir", filepath.Dir(viper.ConfigFileUsed()))
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
		return
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

func initAsync() {
	asyncLevel = viper.GetInt("async")
}

func initLogger() {
	zerolog.DurationFieldUnit = time.Millisecond
	zerolog.DurationFieldInteger = true

	logLevel, err := zerolog.ParseLevel(viper.GetString("log-level"))
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to parse log level")
	}

	// The config is read before the log level is known, so its outcome is logged here.
	zerolog.SetGlobalLevel(logLevel)
	log.Debug().Str("config", viper.ConfigFileUsed()).Str("root-dir", viper.GetString("root-dir")).Str("log-level", logLevel.String()).Msg("Configuration loaded")

	myks.PrintStats = viper.GetBool("print-stats")
}
