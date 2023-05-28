package kwhoosh

import (
	"bytes"
	"io"
	"os/exec"

	"github.com/rs/zerolog/log"
)

type CmdResult struct {
	Stdout string
	Stderr string
}

func runCmd(name string, stdin io.Reader, args []string) (CmdResult, error) {
	log.Debug().Str("cmd", name).Interface("args", args).Msg("Running command")
	cmd := exec.Command(name, args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs

	err := cmd.Run()

	return CmdResult{
		Stdout: stdoutBs.String(),
		Stderr: stderrBs.String(),
	}, err
}

func runYttWithFilesAndStdin(paths []string, stdin io.Reader, args ...string) (CmdResult, error) {
	if stdin != nil {
		paths = append(paths, "-")
	}

	cmdArgs := []string{}
	for _, path := range paths {
		cmdArgs = append(cmdArgs, "--file="+path)
	}

	cmdArgs = append(cmdArgs, args...)
	res, err := runCmd("ytt", stdin, cmdArgs)
	if err != nil {
		log.Warn().Str("cmd", "ytt").Interface("args", cmdArgs).Msg("Failed to run command\n" + res.Stderr)
	}

	return res, err
}

func contains(list []string, item string) bool {
	for _, listItem := range list {
		if listItem == item {
			return true
		}
	}
	return false
}
