package myks

import (
	"maps"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type ChangedFiles map[string]string

// GetChangedFilesGit returns list of files changed since the baseRevision, if specified, and since the last commit
// TODO: exclude files that are outside of the myks root directory
func GetChangedFilesGit(baseRevision string) (ChangedFiles, error) {
	logFn := func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd("collect changed files for smart-mode", name, args)
		if err != nil {
			log.Error().Msg(cmd)
			log.Error().Msg(stderr)

		} else {
			log.Debug().Msg(cmd)
		}
	}

	files := ChangedFiles{}
	if baseRevision != "" {
		result, err := runCmd("git", nil, []string{"diff", "--name-status", baseRevision}, logFn)
		if err != nil {
			return nil, err
		}
		maps.Copy(files, convertToChangedFiles(result.Stdout))
	}

	result, err := runCmd("git", nil, []string{"status", "--porcelain", "--untracked-files"}, logFn)
	if err != nil {
		return nil, err
	}
	maps.Copy(files, convertToChangedFiles(result.Stdout))

	return files, nil
}

func convertToChangedFiles(changes string) ChangedFiles {
	cfs := ChangedFiles{}
	expr := regexp.MustCompile(`^([A-Z]\t|[A-Z? ]{2} )(.*)$`)
	for _, str := range strings.Split(changes, "\n") {
		matches := expr.FindStringSubmatch(str)
		if len(matches) == 3 {
			// 1: the first character of the status, after trimming spaces and tabs
			// 2: the file path
			cfs[matches[2]] = strings.Trim(matches[1], " \t")[:1]
		}
	}
	return cfs
}

func runGitCmd(args []string, root string, silent bool) (string, error) {
	logFn := func(name string, err error, stderr string, args []string) {
		cmd := msgRunCmd("", name, args)
		if err == nil {
			log.Debug().Msg(cmd)
			return
		}
		if silent {
			log.Debug().Msg(cmd)
			log.Debug().Msg(stderr)
			return
		}
		log.Error().Msg(cmd)
		log.Error().Msg(stderr)
	}

	gitArgs := []string{}
	if root != "" {
		gitArgs = append(gitArgs, "-C", root)
	}
	gitArgs = append(gitArgs, args...)
	result, err := runCmd("git", nil, gitArgs, logFn)
	return strings.Trim(result.Stdout, "\n"), err
}

// isGitRepo returns true if the given directory is a git repository
// It does it by running `git rev-parse --git-dir` and checking if it returns an error.
func isGitRepo(root string) bool {
	_, err := runGitCmd([]string{"rev-parse", "--git-dir"}, root, true)
	return err == nil
}

func getGitPathPrefix(root string) (string, error) {
	args := []string{"rev-parse", "--show-prefix"}
	return runGitCmd(args, root, false)
}

func getGitRepoURL(root string) (string, error) {
	args := []string{"remote", "get-url", "origin"}
	return runGitCmd(args, root, false)
}

func getGitRepoBranch(root string) (string, error) {
	args := []string{"rev-parse", "--abbrev-ref", "HEAD"}
	return runGitCmd(args, root, false)
}
