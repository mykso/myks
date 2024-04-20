package embedded

import (
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	carvelCmd "carvel.dev/vendir/pkg/vendir/cmd"
	uierrs "github.com/cppforlife/go-cli-ui/errors"
	"github.com/cppforlife/go-cli-ui/ui"
)

// copied from https://github.com/mykso/vendir/blob/unique-tmp-dir/cmd/vendir/vendir.go
func vendirMain() {
	rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

	log.SetOutput(io.Discard)

	// TODO logs
	// TODO log flags used

	confUI := ui.NewConfUI(ui.NewNoopLogger())
	defer confUI.Flush()

	command := carvelCmd.NewDefaultVendirCmd(confUI)

	err := command.Execute()
	if err != nil {
		confUI.ErrorLinef("vendir: Error: %v", uierrs.NewMultiLineError(err))
		os.Exit(1)
	}

	confUI.PrintLinef("Succeeded")
}
