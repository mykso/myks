package myks

import (
	"github.com/rs/zerolog/log"
)

type SyncTool interface {
	Ident() string
	Sync(a *Application, secrets string) error
	GenerateSecrets(g *Globe) (string, error)
}

func (a *Application) Sync(syncTool SyncTool, secrets string) error {
	err := syncTool.Sync(a, secrets)
	if err != nil {
		log.Error().Err(err).Msg(a.Msg(syncTool.Ident(), "Failed during sync"))
		return err
	}
	return nil
}
