package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v3"
)

func init() {
	cmd := &cobra.Command{
		Use:   "print-config",
		Short: "Print myks configuration",
		Long:  "Print myks configuration",
		Run: func(cmd *cobra.Command, args []string) {
			c := viper.AllSettings()
			bs, err := yaml.Marshal(c)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to marshal config")
			}
			fmt.Printf("---\n%s\n", bs)
		},
	}
	rootCmd.AddCommand(cmd)
}
