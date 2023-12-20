package myks

import (
	"fmt"
	"strconv"
)

type Plugin interface {
	Exec(a *Application) error
	Name() string
}

var _ Plugin = &PluginCmd{}

func NewPluginFromCmd(cmd string) Plugin {
	return &PluginCmd{
		name: cmd,
		cmd:  cmd,
	}
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
	// TODO: Execute command with env
	prefix := "[" + p.name + "] " + a.e.Id + "/" + a.Name + ": "
	fmt.Printf("%s%s\n", prefix, "executing...")
	fmt.Printf("%s%#v\n", prefix, env)
	return nil
}
