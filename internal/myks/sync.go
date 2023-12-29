package myks

import (
	_ "embed"

	"github.com/rs/zerolog/log"
)

type YamlSyncTool interface {
	Ident() string
	Sync(a *Application, secrets string) error
	GenerateSecrets(g *Globe) (string, error)
}

func (a *Application) Sync(yamlSyncTool YamlSyncTool, secrets string) error {
	err := yamlSyncTool.Sync(a, secrets)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(yamlSyncTool.Ident(), "Failed during sync step: "+yamlSyncTool.Ident()))
		return err
	}
	return nil
}
