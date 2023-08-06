package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"

	"github.com/mykso/myks/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}
