package myks

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/quick"
	aurora "github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

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

func printFileNicely(name, content, syntax string) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Println(content)
		return
	}

	fmt.Println(aurora.Bold(fmt.Sprintf("=== %s ===\n", name)))
	err := quick.Highlight(os.Stdout, content, syntax, "terminal16m", "doom-one2")
	if err != nil {
		log.Error().Err(err).Msg("Failed to highlight")
	} else {
		fmt.Printf("\n\n")
	}
}

func process[Item any](asyncLevel int, collection iter.Seq[Item], fn func(Item) error) error {
	var eg errgroup.Group
	if asyncLevel == 0 {
		// no limit
		asyncLevel = -1
	}
	eg.SetLimit(asyncLevel)

	for item := range collection {
		// Create a new variable to avoid capturing the same item in the closure
		innerItem := item
		eg.Go(func() error {
			return fn(innerItem)
		})
	}

	return eg.Wait()
}

func runCmd(step, name string, stdin io.Reader, args []string, metrics *MetricsManager, logFn func(name string, err error, stderr string, args []string)) (CmdResult, error) {
	cmd := exec.Command(name, args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs

	start := time.Now()
	err := cmd.Run()
	metrics.TrackCmdMetric(step, cmd, time.Since(start))

	if logFn != nil {
		logFn(name, err, stderrBs.String(), args)
	}

	return CmdResult{
		Stdout: stdoutBs.String(),
		Stderr: stderrBs.String(),
	}, err
}

func msgRunCmd(purpose string, cmd string, args []string) string {
	msg := cmd + " " + strings.Join(reductSecrets(args), " ")
	if purpose == "" {
		return "Ran \u001B[34m" + cmd + "\u001B[0m\n\u001B[37m" + msg + "\u001B[0m"
	} else {
		return "Ran \u001B[34m" + cmd + "\u001B[0m to: \u001B[3m" + purpose + "\u001B[0m\n\u001B[37m" + msg + "\u001B[0m"
	}
}

func runYttWithFilesAndStdin(step string, paths []string, stdin io.Reader, metrics *MetricsManager, logFn func(name string, err error, stderr string, args []string), args ...string) (CmdResult, error) {
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
	return runCmd(step, myksFullPath(), stdin, cmdArgs, metrics, logFn)
}

func createURLSlug(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "oci://")
	url = strings.ReplaceAll(url, "/", "-")
	return url
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

func msgWithSteps(step1 string, step2 string, msg string) string {
	return fmt.Sprintf(GlobalExtendedLogFormat, step1, step2, msg)
}
