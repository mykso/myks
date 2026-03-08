package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/mykso/myks/cmd"
	embedded "github.com/mykso/myks/cmd/embedded"
	"github.com/mykso/myks/internal/myks"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if embedded.CheckAndRun() {
		return
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	defer myks.PrintCmdMetrics()
	err := cmd.NewMyksCmd(version, commit, date).Execute()
	if err != nil {
		log.Error().Err(err).Msg("Error executing myks")
		os.Exit(1)
	}
}
