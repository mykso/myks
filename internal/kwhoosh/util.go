package kwhoosh

import (
	"bytes"
	"os/exec"

	"github.com/rs/zerolog/log"
)

type CmdResult struct {
	Stdout string
	Stderr string
}

// Process a list of files with ytt and return the result as a string
func YttFiles(paths []string) (CmdResult, error) {
	args := []string{}
	for _, path := range paths {
		args = append(args, "--file="+path)
	}
	log.Debug().Interface("args", args).Msg("ytt args")
	cmd := exec.Command("ytt", args...)

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs

	err := cmd.Run()

	return CmdResult{
		Stdout: stdoutBs.String(),
		Stderr: stderrBs.String(),
	}, err
}
