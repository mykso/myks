package myks

import (
	"bytes"
	_ "embed"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

//go:embed templates/argocd/appproject.ytt.yaml
var argocd_appproject_template []byte

func (e *Environment) renderArgoCD() (err error) {
	if !e.argoCDEnabled {
		log.Debug().Msg(e.Msg("ArgoCD is disabled"))
		return
	}

	// 0. Global data values schema and library files are added later in the a.yttS call
	// 1. Collection of environment main data values and schemas
	yttFiles := e.collectBySubpath(e.g.EnvironmentDataFileName)
	// 2. Collection of environment argocd-specific data values and schemas, and overlays
	yttFiles = append(yttFiles, e.collectBySubpath(filepath.Join("_env", e.g.ArgoCDDataDirName))...)

	res, err := e.yttS(
		"create ArgoCD project yaml",
		yttFiles,
		bytes.NewReader(argocd_appproject_template),
	)
	if err != nil {
		log.Error().Err(err).Str("stdout", res.Stdout).Str("stderr", res.Stderr).Msg(e.Msg("failed to render ArgoCD project yaml"))
		return
	}

	filepath := filepath.Join(e.getArgoCDDestinationDir(), "appproject.yaml")
	err = writeFile(filepath, []byte(res.Stdout))
	if err != nil {
		return
	}

	return
}

func (e *Environment) getArgoCDDestinationDir() string {
	return filepath.Join(e.g.RootDir, e.g.RenderedDir, "argocd", e.Id)
}
