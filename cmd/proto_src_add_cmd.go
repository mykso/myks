package cmd

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mykso/myks/internal/myks"
	"github.com/mykso/myks/internal/prototypes"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func newProtoAddSrcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add prototype src",
		Long:  `Create a new prototype or extend an existing.`,
		Run: func(cmd *cobra.Command, args []string) {
			prototype, err := cmd.Flags().GetString("prototype")
			cobra.CheckErr(err)
			if prototype == "" {
				cobra.CheckErr("Protoype must be provided")
			}
			name, err := cmd.Flags().GetString("name")
			cobra.CheckErr(err)
			if name == "" {
				cobra.CheckErr("Name must be provided")
			}
			create, err := cmd.Flags().GetBool("create")
			cobra.CheckErr(err)
			kind, err := cmd.Flags().GetString("kind")
			cobra.CheckErr(err)
			if kind == "" || (kind != "ytt" && kind != "helm" && kind != "static" && kind != "ytt-pkg") {
				cobra.CheckErr("Kind must be one of ytt, helm, static, ytt-pkg")
			}
			source, err := cmd.Flags().GetString("source")
			cobra.CheckErr(err)
			if source == "" || (source != "git" && source != "helmChart") {
				cobra.CheckErr("Source must be one of git, helmChart")
			}
			uri, err := cmd.Flags().GetString("url")
			cobra.CheckErr(err)
			if uri == "" {
				cobra.CheckErr("URL must be provided")
			}
			_, err = url.ParseRequestURI(uri)
			cobra.CheckErr(err)
			version, err := cmd.Flags().GetString("version")
			cobra.CheckErr(err)
			if version == "" {
				cobra.CheckErr("Version must be provided")
			}
			newRootPath, err := cmd.Flags().GetString("newRootPath")
			cobra.CheckErr(err)
			includes, err := cmd.Flags().GetStringSlice("include")
			cobra.CheckErr(err)

			// start
			g := myks.New(".")

			file := prototype
			if !strings.HasPrefix(prototype, g.PrototypesDir) {
				file = filepath.Join(g.PrototypesDir, prototype)
			}
			if !strings.HasSuffix("vendir/vendir-data.ytt.yaml", file) {
				file = filepath.Join(file, "vendir/vendir-data.ytt.yaml")
			}

			p, err := prototypes.Load(file)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Err(err).Str("prototype", file).Msg("Invalid prototype file")
					cobra.CheckErr(err)
				}
				if !create {
					log.Err(err).Str("prototype", file).Msg("Prototype file does not exist")
					cobra.CheckErr(err)
				}
				log.Info().Str("prototype", file).Msg("Create new prototype")
				p, err = prototypes.Create(file)
				cobra.CheckErr(err)
			}
			protoSrcName := filepath.Base(name)
			p.AddPrototype(prototypes.Prototype{
				Name:         protoSrcName,
				Kind:         prototypes.Kind(kind),
				Source:       prototypes.Source(source),
				Url:          uri,
				Version:      version,
				NewRootPath:  newRootPath,
				IncludePaths: includes,
			})
			err = p.Save()
			cobra.CheckErr(err)
			log.Info().Str("prototype", file).Msg("Prototype added")
		},
	}

	cmd.Flags().StringP("prototype", "p", "", "Name of prototype to manage")
	cmd.Flags().StringP("name", "n", "", "Name of prototype, may include folder")
	cmd.Flags().StringP("kind", "k", "helm", "Kind of prototype")
	cmd.Flags().StringP("source", "s", "git", "Source of prototype")
	cmd.Flags().StringP("url", "u", "", "URL of prototype")
	cmd.Flags().StringP("version", "v", "", "Version of prototype")
	cmd.Flags().StringP("newRootPath", "r", "", "New root path of prototype")
	cmd.Flags().StringSliceP("include", "i", []string{}, "Include files")
	cmd.Flags().BoolP("create", "c", false, "Create new prototype if not exists")

	cmd.MarkFlagRequired("prototype")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("url")
	cmd.MarkFlagRequired("version")

	return cmd
}
