package myks

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type ChangedFiles map[string]string

// getChangedFiles returns list of files changed since the baseRevision, if specified, and since the last commit
func getChangedFiles(baseRevision string) (ChangedFiles, error) {
	logFn := func(name string, args []string) {
		log.Debug().Msg(msgRunCmd("collect changed files for smart-mode", name, args))
	}

	files := ChangedFiles{}
	if baseRevision != "" {
		result, err := runCmd("git", nil, []string{"diff", "--name-status", baseRevision}, logFn)
		if err != nil {
			return nil, err
		}
		for path, status := range convertToChangedFiles(result.Stdout) {
			files[path] = status
		}
	}

	// add all tracked modified files
	result, err := runCmd("git", nil, []string{"status", "--porcelain"}, logFn)
	if err != nil {
		return nil, err
	}
	for path, status := range convertToChangedFiles(result.Stdout) {
		if status != "?" {
			files[path] = status
		}
	}

	// add all untracked files
	result, err = runCmd("git", nil, []string{"ls-files", "--others", "--exclude-standard"}, logFn)
	if err != nil {
		return nil, err
	}
	for path, status := range addUntrackedFiles(result.Stdout) {
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

func addUntrackedFiles(additions string) ChangedFiles {
	cfs := ChangedFiles{}
	for _, path := range strings.Split(additions, "\n") {
		if path != "" {
			cfs[path] = "U"
		}
	}
	return cfs
}
