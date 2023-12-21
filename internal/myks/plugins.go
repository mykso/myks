package myks

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type Plugin interface {
	Exec(a *Application) error
	Name() string
}

var _ Plugin = &PluginCmd{}

const pluginPrefix = "myks-"

func NewPluginFromCmd(cmd string) Plugin {
	name := strings.TrimPrefix(filepath.Base(cmd), pluginPrefix)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return &PluginCmd{
		name: name,
		cmd:  cmd,
	}
}

// FindPluginsInPaths searches for plugins in the specified paths.
// If no paths are provided, it uses the directories listed in the PATH environment variable.
// It returns a slice of Plugin objects representing the found plugins.
func FindPluginsInPaths(paths []string) []Plugin {
	var plugins []Plugin
	if len(paths) == 0 {
		paths = filepath.SplitList(os.Getenv("PATH"))
	}
	for _, path := range paths {
		if len(strings.TrimSpace(path)) == 0 {
			continue
		}
		files, err := os.ReadDir(path)
		if err != nil {
			log.Debug().Err(err).Msgf("Unable to read directory %s", path)
			continue
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if !strings.HasPrefix(file.Name(), pluginPrefix) {
				continue
			}
			if isExec, err := isExecutable(filepath.Join(path, file.Name())); err != nil {
				log.Debug().Err(err).Msgf("Unable to check if %s is executable", filepath.Join(path, file.Name()))
			} else if !isExec {
				log.Trace().Msgf("Skipping %s because it is not executable", filepath.Join(path, file.Name()))
				continue
			}
			plugins = append(plugins, NewPluginFromCmd(filepath.Join(path, file.Name())))
		}
	}
	return plugins
}

func isExecutable(fullPath string) (bool, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return false, err
	}

	if runtime.GOOS == "windows" {
		fileExt := strings.ToLower(filepath.Ext(fullPath))

		switch fileExt {
		case ".bat", ".cmd", ".com", ".exe", ".ps1":
			return true, nil
		}
		return false, nil
	}

	if m := info.Mode(); !m.IsDir() && m&0111 != 0 {
		return true, nil
	}

	return false, nil
}

type PluginCmd struct {
	name string
	cmd  string
}

func (p PluginCmd) Name() string {
	return p.name
}

func (p PluginCmd) Exec(a *Application) error {
	env := map[string]string{
		"MYKS_ENV":              a.e.Id,
		"MYKS_APP":              a.Name,
		"MYKS_APP_PROTOTYPE":    a.Prototype,
		"MYKS_ENV_DIR":          a.e.Dir,
		"MYKS_ENV_DATA_FILE":    a.e.renderedEnvDataFilePath,
		"MYKS_RENDERED_APP_DIR": "rendered/envs/" + a.e.Id + "/" + a.Name, // TODO: provide func and use it everywhere,
		"MYKS_ARGOCD_ENABLED":   strconv.FormatBool(a.argoCDEnabled),
	}
	if a.argoCDEnabled {
		env["MYKS_ARGOCD_APP_NAME"] = a.getArgoCDDestinationDir()
		env["MYKS_ARGOCD_APP_PROJECT"] = "rendered/argocd/" + a.e.Id + "/" + a.Name // TODO: provide func and use it everywhere,
	}
	prefix := "[" + p.name + "] " + a.e.Id + "/" + a.Name + ": "
	log.Info().Msg(prefix + "Executing plugin")

	cmd := exec.Command(p.cmd, []string{}...)

	var stdoutBs, stderrBs bytes.Buffer
	cmd.Stdout = &stdoutBs
	cmd.Stderr = &stderrBs
	cmd.Env = append(os.Environ(), mapToSlice(env)...)

	err := cmd.Run()
	if err != nil {
		log.Error().Err(err).Msg(prefix + "Plugin execution failed")
		log.Info().Msg(prefix + "Stdout: " + stdoutBs.String())
		log.Info().Msg(prefix + "Stderr: " + stderrBs.String())
	}
	return err
}
