package embedded

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"carvel.dev/ytt/pkg/cmd"
	uierrs "github.com/cppforlife/go-cli-ui/errors"
)

// copied from https://github.com/carvel-dev/ytt/blob/develop/cmd/ytt/ytt.go
func yttMain() {
	rand.Seed(time.Now().UTC().UnixNano())

	command := cmd.NewDefaultYttCmd()

	err := command.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ytt: Error: %s\n", uierrs.NewMultiLineError(err))
		os.Exit(1)
	}
}
