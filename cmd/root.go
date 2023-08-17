package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile            string
	targetEnvironments []string
	targetApplications []string
	asyncLevel         int
)

var rootCmd = &cobra.Command{
	Use:   "myks",
	Short: "Myks helps to manage configuration for kubernetes clusters",
	Long:  "Myks TBD",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Check positional arguments:
		// 1. Comma-separated list of environment search paths or ALL to search everywhere (default: ALL)
		// 2. Comma-separated list of application names or none to process all applications (default: none)

		targetEnvironments = nil
		targetApplications = nil

		onlyArgs := args

		for _, subcmd := range cmd.Commands() {
			if subcmd.Name() == args[0] {
				onlyArgs = args[1:]
				break
			}
		}

		switch len(onlyArgs) {
		case 0:
			// No positional arguments
		case 1:
			if onlyArgs[0] != "ALL" {
				targetEnvironments = strings.Split(onlyArgs[0], ",")
			}
		case 2:
			if onlyArgs[0] != "ALL" {
				targetEnvironments = strings.Split(onlyArgs[0], ",")
			}
			targetApplications = strings.Split(onlyArgs[1], ",")
		default:
			err := errors.New("Too many positional arguments")
			log.Error().Err(err).Msg("Unable to parse positional arguments")
			return err
		}

		log.Debug().Strs("environments", targetEnvironments).Strs("applications", targetApplications).Msg("Parsed arguments")

		return nil
	},
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "Set the logging level")
	rootCmd.PersistentFlags().IntVarP(&asyncLevel, "async", "a", 0, "Sets the number of concurrent processed applications. The default is no limit.")

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.Fatal().Err(err).Msg("Unable to bind flags")
	}
	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		log.Fatal().Err(err).Msg("Unable to bind flags")
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is the first .myks.yaml up the directory tree)")
}

func initConfig() {
	viper.SetEnvPrefix("MYKS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName(".myks")
		viper.SetConfigType("yaml")

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
	initLogger()

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

func SetVersionInfo(version, commit, date string) {
	rootCmd.Version = fmt.Sprintf(`%s
     commit: %s
     date:   %s`, version, commit, date)
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
