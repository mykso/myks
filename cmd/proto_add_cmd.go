package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mykso/myks/internal/myks"
	"github.com/mykso/myks/internal/prototypes"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	protoCmd.AddCommand(newProtoAddCmd())
}

func newProtoAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add prototype",
		Long:  `Create a new prototype or extend an existing.`,
		Run: func(cmd *cobra.Command, args []string) {
			name, err := cmd.Flags().GetString("name")
			cobra.CheckErr(err)
			if name == "" {
				cobra.CheckErr("Name must be provided")
			}
			// start
			g := myks.New(".")

			file := name
			if !strings.HasPrefix(name, g.PrototypesDir) {
				file = filepath.Join(g.PrototypesDir, name)
			}
			if !strings.HasSuffix("vendir/vendir-data.ytt.yaml", file) {
				file = filepath.Join(file, "vendir/vendir-data.ytt.yaml")
			}
			err = os.MkdirAll(filepath.Dir(file), os.ModePerm)
			cobra.CheckErr(err)

			p, err := prototypes.Create(file)
			cobra.CheckErr(err)

			err = p.Save()
			cobra.CheckErr(err)
			log.Info().Str("prototype", file).Msg("Prototype create")
		},
	}

	cmd.Flags().StringP("name", "n", "", "Name of prototype, may include folder")
	cmd.MarkFlagRequired("name")

	return cmd
}
