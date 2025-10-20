package embedded

import (
	"io"
	"log"
	"os"

	carvelCmd "carvel.dev/kbld/pkg/kbld/cmd"
	uierrs "github.com/cppforlife/go-cli-ui/errors"
	"github.com/cppforlife/go-cli-ui/ui"
)

func kbldMain() {
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
}
