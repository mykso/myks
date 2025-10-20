// Package embedded provides functionality to run embedded commands like ytt and vendir.
package embedded

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// CheckAndRun checks if the first argument is an embedded command and runs it.
// It returns true if an embedded command was run.
func CheckAndRun() bool {
	if len(os.Args) < 2 {
		return false
	}

	var embeddedF func()
	switch os.Args[1] {
	case "kbld":
		embeddedF = kbldMain
	case "ytt":
		embeddedF = yttMain
	case "vendir":
		embeddedF = vendirMain
	default:
		return false
	}

	os.Args = os.Args[1:]
	embeddedF()
	return true
}

func EmbeddedCmd(name string, description string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Run embedded %s", name),
		Long:  description,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("build error with embedded '%s'. Please create an issue at https://github.com/mykso/myks/issues/new", name)
		},
	}
}
