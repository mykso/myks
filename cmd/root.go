package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "kwhoosh",
	Short: "Kwhoosh helps to manage configuration for kubernetes clusters",
	Long:  "Kwhoosh TBD",
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kwhoosh.yaml)")
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", "Set the logging level")

	viper.BindPFlags(rootCmd.PersistentFlags())
	viper.BindPFlags(rootCmd.Flags())
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
