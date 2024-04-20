package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/mykso/myks/cmd"
	embedded "github.com/mykso/myks/cmd/embedded"
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
	cmd := cmd.NewMyksCmd(version, commit, date)
	err := cmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Error executing myks")
	}
}
