package cmd

import (
	"errors"
	"os"
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
)

var rootCmd = &cobra.Command{
	Use:   "kwhoosh",
	Short: "Kwhoosh helps to manage configuration for kubernetes clusters",
	Long:  "Kwhoosh TBD",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Check positional arguments:
		// 1. Comma-separated list of envirmoment search paths or ALL to search everywhere (default: ALL)
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

	if err := viper.BindPFlags(rootCmd.PersistentFlags()); err != nil {
		log.Fatal().Err(err).Msg("Unable to bind flags")
	}
	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		log.Fatal().Err(err).Msg("Unable to bind flags")
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kwhoosh.yaml)")
}

func initConfig() {
	viper.SetEnvPrefix("KWH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("$HOME/.kwhoosh")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")

		xdgConfigHome, err := os.UserConfigDir()
		if err != nil {
			log.Warn().Err(err).Msg("Unable to determine XDG_CONFIG_HOME")
		} else {
			viper.AddConfigPath(xdgConfigHome + "/kwhoosh")
		}
	}

	err := viper.ReadInConfig()
	initLogger()

	if err == nil {
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

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
