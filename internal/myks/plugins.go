package myks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog/log"
)

type Plugin interface {
	Exec(a *Application, args []string, bufferOutput bool) error
	Name() string
}

type PluginCmd struct {
	name string
	cmd  string
}

// Ensure PluginCmd implements the Plugin interface
var _ Plugin = &PluginCmd{}

func NewPluginFromCmd(cmd, filePrefix string) Plugin {
	name := strings.TrimPrefix(filepath.Base(cmd), filePrefix)
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return &PluginCmd{
		name: name,
		cmd:  cmd,
	}
}

// FindPluginsInPaths searches for plugins in the specified paths.
// It returns a slice of Plugin objects representing the found plugins.
func FindPluginsInPaths(paths []string, filePrefix string) []Plugin {
	var plugins []Plugin
	if len(paths) == 0 {
		return plugins
	}
	for _, path := range paths {
		if len(strings.TrimSpace(path)) == 0 {
			continue
		}
		files, err := os.ReadDir(path)
		if err != nil {
			log.Trace().Err(err).Msgf("Unable to read directory %s", path)
			continue
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			if !strings.HasPrefix(file.Name(), filePrefix) {
				continue
			}
			executable := filepath.Join(path, file.Name())
			if isExec, err := isExecutable(executable); err != nil {
				log.Debug().Err(err).Msgf("Unable to check if %s is executable", executable)
			} else if !isExec {
				log.Trace().Msgf("Skipping %s because it is not executable", executable)
				continue
			}
			plugins = append(plugins, NewPluginFromCmd(executable, filePrefix))
		}
	}
	return plugins
}

func isExecutable(fullPath string) (bool, error) {
	if runtime.GOOS == "windows" {
		fileExt := strings.ToLower(filepath.Ext(fullPath))

		switch fileExt {
		case ".bat", ".cmd", ".com", ".exe", ".ps1":
			return true, nil
		}
		return false, nil
	}

	if info, err := os.Stat(fullPath); err != nil {
		return false, err
	} else if m := info.Mode(); !m.IsDir() && m&0o111 != 0 {
		return true, nil
	}

	return false, nil
}

func (p PluginCmd) Name() string {
	return p.name
}

func (p PluginCmd) Exec(a *Application, args []string, bufferOutput bool) error {
	step := p.Name()
	log.Trace().Msg(a.Msg(step, "execution started"))

	env, err := p.generateEnv(a)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(step, "Generating data values failed"))
		return err
	}

	cmd := exec.Command(p.cmd, args...) // #nosec G204 -- this is a user-provided command
	cmd.Env = append(os.Environ(), mapToSlice(env)...)
	log.Debug().Msg(a.Msg(step, msgRunCmd("", p.cmd, args)))

	if bufferOutput {
		// Buffered mode: capture output and print after completion
		var stdoutBs, stderrBs bytes.Buffer
		cmd.Stdout = &stdoutBs
		cmd.Stderr = &stderrBs
		err = cmd.Run()
		fmt.Println(stdoutBs.String())
		if err != nil {
			log.Error().Err(err).Msg(msgRunCmd("External plugin failed: "+step, p.cmd, args))
			if stderrBs.Len() > 0 {
				log.Error().Msg(stderrBs.String())
			}
		} else {
			log.Info().Msg(msgRunCmd("External plugin succeeded: "+step, p.cmd, args))
			if stderrBs.Len() > 0 {
				log.Info().Msg(stderrBs.String())
			}
		}
	} else {
		// Streaming mode: output in real-time
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Error().Err(err).Msg(msgRunCmd("External plugin failed: "+step, p.cmd, args))
		} else {
			log.Info().Msg(msgRunCmd("External plugin succeeded: "+step, p.cmd, args))
		}
	}
	return err
}

func (p PluginCmd) generateEnv(a *Application) (map[string]string, error) {
	env := map[string]string{
		"MYKS_ENV":              a.e.ID,
		"MYKS_APP":              a.Name,
		"MYKS_APP_PROTOTYPE":    a.Prototype,
		"MYKS_ENV_DIR":          a.e.Dir,
		"MYKS_RENDERED_APP_DIR": a.getDestinationDir(),
	}

	result, err := a.ytt(p.Name(), "get data values", a.yttDataFiles, "--data-values-inspect")
	if err == nil {
		env["MYKS_DATA_VALUES"] = result.Stdout
	}
	return env, err
}
