package myks

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

type ChangedFile struct {
	path   string
	status string
}

// return all files paths that were changed since the revision
func getChangedFiles(revision string) ([]ChangedFile, error) {
	logFn := func(name string, args []string) {
		log.Debug().Msg(msgRunCmd("get diff for smart-mode", name, args))
	}
	_, err := runCmd("git", nil, []string{"add", ".", "--intent-to-add"}, logFn)
	if err != nil {
		return nil, err
	}
	result, err := runCmd("git", nil, []string{"diff", "--ignore-blank-lines", "--name-status", revision}, logFn)
	if err != nil {
		return nil, err
	}
	if result.Stdout == "" {
		return nil, nil
	}
	return convertToChangedFiles(result.Stdout), err
}

func convertToChangedFiles(changes string) []ChangedFile {
	var cfs []ChangedFile
	for _, str := range strings.Split(changes, "\n") {
		if str != "" {
			parts := strings.Split(str, "\t")
			cf := ChangedFile{path: parts[1], status: parts[0]}
			cfs = append(cfs, cf)
		}
	}
	return cfs
}

func extractChangedFilePaths(cfs []ChangedFile) []string {
	var paths []string
	for _, cf := range cfs {
		paths = append(paths, cf.path)
	}
	return paths
}

func extractChangedFilePathsWithStatus(cfs []ChangedFile, status string) []string {
	filter := func(cf ChangedFile) bool {
		if status == "" || cf.status == status {
			return true
		}
		return false
	}
	return extractChangedFilePaths(extract(cfs, filter))
}

func extractChangedFilePathsWithoutStatus(cfs []ChangedFile, status string) []string {
	filter := func(cf ChangedFile) bool {
		if status == "" || cf.status != status {
			return true
		}
		return false
	}
	return extractChangedFilePaths(extract(cfs, filter))
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
