package proto

import (
	"github.com/mykso/myks/internal/myks"
	"github.com/mykso/myks/internal/prototypes"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// updates the version of a prototype source
func newProtoBumpCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:               "bump",
		Short:             "bump versions",
		Long:              `Check for new versions of prototype sources and updates them.`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: prototypeCompletion,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			g := myks.New(".")

			protos := args
			if protos[0] == "all" {
				protos, err = prototypes.CollectPrototypes(g)
				cobra.CheckErr(err)
			}

			for _, p := range protos {
				proto, err := prototypes.Load(g, p)
				if err != nil {
					log.Err(err).Str("prototype", p).Msg("Failed to load prototype")
					continue
				}

				for _, s := range proto.Sources {
					change, err := proto.Bump(s)
					if err != nil {
						log.Err(err).Str("prototype", p).Str("source", s.Name).Msg("Failed to bump prototype")
						continue
					}
					switch change {
					case prototypes.Bumped:
						log.Info().Str("prototype", p).Str("source", s.Name).Msg("Prototype bumped")
						err = proto.Save()
						cobra.CheckErr(err)
					case prototypes.UpToDate:
						log.Debug().Str("prototype", p).Msg("Prototype is up to date")
					case prototypes.Unsupported:
						log.Warn().Str("prototype", p).Str("source", s.Name).Str("repo", string(s.Repo)).Msg("update not supported")
					}

				}
			}
		},
	}

	return cmd
}
