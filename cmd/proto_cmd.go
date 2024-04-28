package cmd

import "github.com/spf13/cobra"

var protoCmd = &cobra.Command{
	Use:   "proto",
	Short: "Manage prototypes",
	Long:  `Manage prototypes with myks.`,
}
