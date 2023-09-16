package myks

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type ChangedFiles map[string]string

// getChangedFiles returns list of files changed sinc the revision, if specified, and since the last commit
func getChangedFiles(revision string) (ChangedFiles, error) {
	logFn := func(name string, args []string) {
		log.Debug().Msg(msgRunCmd("get diff for smart-mode", name, args))
	}

	files := ChangedFiles{}
	if revision != "" {
		result, err := runCmd("git", nil, []string{"diff", "--name-status", revision}, logFn)
		if err != nil {
			return nil, err
		}
		for path, status := range convertToChangedFiles(result.Stdout) {
			files[path] = status
		}
	}

	result, err := runCmd("git", nil, []string{"status", "--porcelain"}, logFn)
	if err != nil {
		return nil, err
	}
	for path, status := range convertToChangedFiles(result.Stdout) {
		files[path] = status
	}

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

// get head revision of main branch
func getMainBranchHeadRevision(mainBranch string) (string, error) {
	logFn := func(name string, args []string) {
		log.Debug().Msg(msgRunCmd("get main branch head revision for smart-mode", name, args))
	}
	_, err := runCmd("git", nil, []string{"fetch", "origin", mainBranch}, logFn)
	if err != nil {
		return "", err
	}
	cmdResult, err := runCmd("git", nil, []string{"merge-base", "origin/" + mainBranch, "HEAD"}, logFn)
	if err != nil {
		return "", err
	}
	// git adds new line to output which messes up the result
	headRevision := strings.TrimRight(cmdResult.Stdout, "\n")
	return headRevision, nil
}

// get head revision
func getCurrentBranchHeadRevision() (string, error) {
	logFn := func(name string, args []string) {
		log.Debug().Msg(msgRunCmd("get current head revision for smart-mode", name, args))
	}
	cmdResult, err := runCmd("git", nil, []string{"rev-parse", "HEAD"}, logFn)
	if err != nil {
		return "", fmt.Errorf("failed to get current branch head revision: %v", err)
	}
	return strings.TrimRight(cmdResult.Stdout, "\n"), nil
}

func getDiffRevision(mainBranch string) (string, error) {
	if os.Getenv("CI") != "" {
		log.Debug().Msg("Pipeline mode: comparing with HEAD revision on main")
		return getMainBranchHeadRevision(mainBranch)
	}
	log.Debug().Msg("Local mode: comparing with HEAD revision on current branch")
	return getCurrentBranchHeadRevision()
}
