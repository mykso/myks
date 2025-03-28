package myks

import (
	"maps"
	"strings"

	"github.com/rs/zerolog/log"
)

type ChangedFiles map[string]string

// GetChangedFilesGit returns list of files changed since the baseRevision, if specified, and since the last commit
// GetChangedFilesGit retrieves a mapping of file paths to their change modes for all modified files in the Git repository.
// When a non-empty baseRevision is provided, it uses "git diff --name-status -z" to capture changes since that revision.
// It also runs "git status -z --untracked-files" to incorporate untracked file changes, merging both outputs into a single ChangedFiles map.
// Returns the consolidated ChangedFiles map or an error if any Git command execution fails.
// TODO: Exclude files that are outside of the myks root directory.
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
		result, err := runCmd("git", nil, []string{"diff", "--name-status", "-z", baseRevision}, logFn)
		if err != nil {
			return nil, err
		}
		maps.Copy(files, convertDiffToChangedFiles(result.Stdout))
	}

	result, err := runCmd("git", nil, []string{"status", "-z", "--untracked-files"}, logFn)
	if err != nil {
		return nil, err
	}
	maps.Copy(files, convertStatusToChangedFiles(result.Stdout))

	return files, nil
}

// convertDiffToChangedFiles parses the null-terminated output from `git diff --name-status -z` and constructs a map that associates each file path with its change mode.
// The input string alternates between a status code (identified by its first character) and the corresponding file path.
// For rename changes (indicated by 'R'), which involve two file paths, both entries are recorded with the rename status.
func convertDiffToChangedFiles(diff string) ChangedFiles {
	files := ChangedFiles{}
	mode := ""
	// If the second file in a rename is being processed
	second := false
	for _, str := range strings.Split(diff, "\x00") {
		if str == "" {
			continue
		}
		if mode == "" {
			mode = str[:1]
			continue
		}
		files[str] = mode
		// The rename mode is processed differently, as two files are involved
		if mode != "R" || second {
			mode = ""
			second = false
		} else if mode == "R" {
			second = true
		}
	}
	return files
}

// convertStatusToChangedFiles parses the null-separated output from "git status -z --untracked-files" into a ChangedFiles map,
// where each key is a file path and each value is the corresponding change mode.
// For rename operations (indicated by "R"), the function expects two consecutive tokens:
// the first signals a rename and the following token provides the new file path.
func convertStatusToChangedFiles(status string) ChangedFiles {
	files := ChangedFiles{}
	mode := ""
	// If the second file in a rename is being processed
	second := false
	for _, str := range strings.Split(status, "\x00") {
		if str == "" {
			continue
		}
		if mode == "R" && second {
			files[str] = mode
			mode = ""
			second = false
			continue
		}
		mode = str[:1]
		files[str[3:]] = mode
		if mode == "R" {
			second = true
		} else {
			mode = ""
		}
	}
	return files
}

// runGitCmd executes a Git command with the provided arguments in an optional working directory.
// If the root is non-empty, it adds the "-C" flag to run the command in that directory.
// The command's standard output is trimmed of trailing newlines and returned along with any error encountered.
// The silent flag controls logging: when true, even errors are logged at the debug level.
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
