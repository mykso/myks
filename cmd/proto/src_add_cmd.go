package proto

import (
	"net/url"
	"os"

	"github.com/mykso/myks/cmd/utils"
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

			repo, err := cmd.Flags().GetString("repo")
			cobra.CheckErr(err)

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
			rootPath, err := cmd.Flags().GetString("rootPath")
			cobra.CheckErr(err)
			includes, err := cmd.Flags().GetStringSlice("include")
			cobra.CheckErr(err)

			// start
			g := myks.New(".")

			p, err := prototypes.Load(g, prototype)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Err(err).Str("prototype", prototype).Msg("Invalid prototype file")
					cobra.CheckErr(err)
				}
				if !create {
					log.Error().Msg("Prototype does not exist. Use --create to create a new prototype")
					return
				}
				p, err = prototypes.Create(g, prototype)
				cobra.CheckErr(err)
				log.Info().Str("prototype", prototype).Msg("Created new prototype")
			}

			if _, exist := p.GetSource(name); exist {
				log.Error().Str("source", name).Msg("Source already exists")
				return
			}
			p.AddSource(prototypes.Source{
				Name:         name,
				Kind:         prototypes.Kind(kind),
				Repo:         prototypes.Repo(repo),
				Url:          uri,
				Version:      version,
				NewRootPath:  rootPath,
				IncludePaths: includes,
			})
			err = p.Save()
			cobra.CheckErr(err)
			log.Info().Str("prototype", prototype).Msg("Prototype source added")
		},
	}

	cmd.Flags().StringP("name", "n", "", "Name of prototype, may include folder")
	cmd.Flags().StringP("url", "u", "", "URL of prototype")
	cmd.Flags().StringP("version", "v", "", "Version of prototype")
	cmd.Flags().String("rootPath", "", "New root path of prototype")
	cmd.Flags().StringSliceP("include", "i", []string{}, "Include files")
	cmd.Flags().BoolP("create", "c", false, "Create new prototype if not exists")

	utils.NewEnumFlag("repo", map[string]string{
		"git":       "Git repository",
		"helmChart": "Helm repository",
	}).EnableFlag(cmd, "repo", "r", "git", "Source repository type")

	utils.NewEnumFlag("kind", map[string]string{
		"ytt":     "Output will be rendered by ytt",
		"helm":    "Output will be rendered by helm template. Requires helm installed.",
		"static":  "Output will be copied as is",
		"ytt-pkg": "Output contains ytt schema and data.",
	}).EnableFlag(cmd, "kind", "k", "helm", "Kind of package")

	cobra.CheckErr(cmd.MarkFlagRequired("name"))
	cobra.CheckErr(cmd.MarkFlagRequired("url"))
	cobra.CheckErr(cmd.MarkFlagRequired("version"))

	return cmd
}
