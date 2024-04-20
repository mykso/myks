package embedded

import (
	"fmt"
	"os"

	"carvel.dev/ytt/pkg/cmd"
	uierrs "github.com/cppforlife/go-cli-ui/errors"
)

// Originated from from https://github.com/carvel-dev/ytt/blob/develop/cmd/ytt/ytt.go
func yttMain() {
	if err := cmd.NewDefaultYttCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ytt: Error: %s\n", uierrs.NewMultiLineError(err))
		os.Exit(1)
	}
}
