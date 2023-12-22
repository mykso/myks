package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mykso/myks/internal/myks"
)

const (
	MYKS_CONFIG_NAME = ".myks"
	MYKS_CONFIG_TYPE = "yaml"
)

var (
	cfgFile    string
	envAppMap  myks.EnvAppMap
	asyncLevel int
)

func NewMyksCmd(version, commit, date string) *cobra.Command {
	cobra.OnInitialize(initLogger)
	cmd := newRootCmd(version, commit, date)
	cmd.AddCommand(allCmd)
	cmd.AddCommand(renderCmd)
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newPrintConfigCmd())
	cmd.AddCommand(newSyncCmd())
	initConfig()
	addPlugins(cmd)
	return cmd
}

func newRootCmd(version, commit, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "myks",
		Short: "Myks helps to manage configuration for kubernetes clusters",
		Long: `Myks fetches K8s workloads from a variety of sources, e.g. Helm charts or Git Repositories. It renders their respective yaml files to the file system in a structure of environments and their applications.
	
	It supports prototype applications that can be shared between environments and inheritance of configuration from parent environments to their "children".
	
	Myks supports two positional arguments:
	
	- A comma-separated list of environments to render. If you provide "ALL", all environments will be rendered.
	- A comma-separated list of applications to render. If you don't provide this argument or provide "ALL", all applications will be rendered.
	
	If you do not provide any positional arguments, myks will run in "Smart Mode". In Smart Mode, myks will only render environments and applications that have changed since the last run.
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

	configHelp := fmt.Sprintf("config file (default is the first %s.%s up the directory tree)", MYKS_CONFIG_NAME, MYKS_CONFIG_TYPE)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", configHelp)

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Annotations[ANNOTATION_SMART_MODE] != ANNOTATION_TRUE {
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
		viper.SetConfigName(MYKS_CONFIG_NAME)
		viper.SetConfigType(MYKS_CONFIG_TYPE)

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
