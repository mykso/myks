package embedded

import (
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	carvelCmd "carvel.dev/kbld/pkg/kbld/cmd"
	uierrs "github.com/cppforlife/go-cli-ui/errors"
	"github.com/cppforlife/go-cli-ui/ui"
)

// copied from https://github.com/carvel-dev/kbld/blob/develop/cmd/kbld/kbld.go
func kbldMain() {
	rand.New(rand.NewSource(time.Now().UTC().UnixNano())) // #nosec G404 -- should be fixed upstream

	log.SetOutput(io.Discard)

	// TODO logs
	// TODO log flags used

	confUI := ui.NewConfUI(ui.NewNoopLogger())
	defer confUI.Flush()

	command := carvelCmd.NewDefaultKbldCmd(confUI)

	err := command.Execute()
	if err != nil {
		confUI.ErrorLinef("kbld: Error: %v", uierrs.NewMultiLineError(err))
		os.Exit(1)
	}

	confUI.PrintLinef("Succeeded")
}
