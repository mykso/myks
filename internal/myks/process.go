package myks

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CmdResult holds the captured stdout and stderr from a command execution.
type CmdResult struct {
	Stdout string
	Stderr string
}

func reductSecrets(args []string) []string {
	sensitiveFields := []string{"password", "secret", "token"}
	var logArgs []string
	for _, arg := range args {
		pattern := "(" + strings.Join(sensitiveFields, "|") + ")=(\\S+)"
		regex := regexp.MustCompile(pattern)
		logArgs = append(logArgs, regex.ReplaceAllString(arg, "$1=[REDACTED]"))
	}
	return logArgs
}

func msgRunCmd(purpose, cmd string, args []string) string {
	msg := cmd + " " + strings.Join(reductSecrets(args), " ")
	if purpose == "" {
		return "Ran \u001B[34m" + cmd + "\u001B[0m\n\u001B[37m" + msg + "\u001B[0m"
	}
	return "Ran \u001B[34m" + cmd + "\u001B[0m to: \u001B[3m" + purpose + "\u001B[0m\n\u001B[37m" + msg + "\u001B[0m"
}

func runCmd(step, name string, stdin io.Reader, args []string, logFn func(name string, err error, stderr string, args []string)) (CmdResult, error) {
	cmd := exec.Command(name, args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs

	start := time.Now()
	err := cmd.Run()
	TrackCmdMetric(step, cmd, time.Since(start))

	if logFn != nil {
		logFn(name, err, stderrBs.String(), args)
	}

	return CmdResult{
		Stdout: stdoutBs.String(),
		Stderr: stderrBs.String(),
	}, err
}

func runYttWithFilesAndStdin(step string, paths []string, stdin io.Reader, logFn func(name string, err error, stderr string, args []string), args ...string) (CmdResult, error) {
	if stdin != nil {
		paths = append(paths, "-")
	}

	cmdArgs := []string{
		"ytt",
	}
	for _, path := range paths {
		cmdArgs = append(cmdArgs, "--file="+path)
	}

	cmdArgs = append(cmdArgs, args...)
	return runCmd(step, myksFullPath(), stdin, cmdArgs, logFn)
}

func myksFullPath() string {
	myks, err := os.Executable()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get myks executable")
	}
	if strings.Contains(myks, ".test") {
		// running go test, the test executable doesn't provide embedded binaries, fallback to myks in PATH
		return "myks"
	}
	return myks
}
