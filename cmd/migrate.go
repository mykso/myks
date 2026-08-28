package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func newMigrateCmd() *cobra.Command {
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convert a legacy ytt data-values repo to the KCL configuration layer",
		Long: `Seed a KCL configuration tree from the legacy ytt data-values files.

The converter writes kcl.mod, main.k and one env.k per environment-tree level:
plain-YAML data-values files are converted in place on the level structure, the
application roster is generated from the resolved environment data, and values
produced by ytt logic are frozen as literals in leaf-level patches marked with
TODO comments.

The legacy files are left untouched; with kcl.mod present, myks switches to the
KCL path and ignores them. Verify the conversion by rendering and diffing the
rendered/ output, hand-finish following docs/migration.md, then delete the
legacy data-values files.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			schemaPackage, err := cmd.Flags().GetString("schema-package")
			okOrFatal(err, "Failed to read flag")
			okOrFatal(myks.Migrate(getGlobe(), schemaPackage), "Migration failed")
		},
	}

	migrateCmd.Flags().String("schema-package", "oci://ghcr.io/mykso/myks",
		"myks KCL schema package to depend on: an oci:// reference or a local path")

	return migrateCmd
}
