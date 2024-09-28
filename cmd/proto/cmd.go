package proto

import "github.com/spf13/cobra"

var Cmd = &cobra.Command{
	Use:   "proto",
	Short: "Manage prototypes",
	Long:  `Manage prototypes with myks.`,
}
