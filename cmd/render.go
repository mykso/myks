package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

func newRenderCmd() *cobra.Command {
	renderCmd := &cobra.Command{
		Use:   "render",
		Short: "Render application manifests",
		Long: `Download external sources and render manifests for specified environments and applications.

Authentication against protected repositories is achieved with environment variables prefixed with "VENDIR_SECRET_".
For example, if you reference a secret named "mycreds" in your vendir.yaml, you need to export the variables "VENDIR_SECRET_MYCREDS_USERNAME" and
"VENDIR_SECRET_MYCREDS_PASSWORD" in your environment.`,
		Args: cobra.RangeArgs(0, 2),
		Annotations: map[string]string{
			AnnotationSmartMode: AnnotationTrue,
		},
		Run: func(cmd *cobra.Command, args []string) {
			sync, syncSet := readFlagBool(cmd, "sync")
			render, renderSet := readFlagBool(cmd, "render")

			if !syncSet && !renderSet {
				sync = true
				render = true
			}

			RenderCmd(sync, render)
		},
		ValidArgsFunction: shellCompletion,
	}

	// Use this template for commands that accept environment and application arguments
	envAppCommandUsageTemplate := `Usage:
  {{.CommandPath}} [environments [applications]] [flags]

Arguments:
  0. When no arguments are provided, myks uses Smart Mode to determine the environments and applications to process.
     In Smart Mode, myks relies on git to only process applications with changes.

  1. environments    (Optional) Comma-separated list of environments or ALL
                     ALL will process all environments
                     Examples: ALL
                               prod,stage,dev
                               prod/region1,stage/region1
                               dev

  2. applications    (Optional) Comma-separated list of applications or ALL
                     ALL will process all applications
                     Example: app1,app2 or ALL

{{if .HasAvailableFlags}}Flags:
{{.Flags.FlagUsages | trimTrailingWhitespaces}}{{end}}

Examples:
  # Process all apps in production and staging
  {{.CommandPath}} prod,stage ALL

  # Process specific apps in all environments
  {{.CommandPath}} ALL app1,app2

  # Process specific apps in specific environments
  {{.CommandPath}} prod,stage app1,app2
`

	renderCmd.SetUsageTemplate(envAppCommandUsageTemplate)

	renderCmd.Flags().BoolP("sync", "s", false, "only sync external sources")
	renderCmd.Flags().BoolP("render", "r", false, "only render manifests")
	renderCmd.MarkFlagsMutuallyExclusive("sync", "render")

	return renderCmd
}

// RenderCmd processes the render command with the provided flags.
// The function is exported to allow testing and usage in other packages.
func RenderCmd(sync, render bool) {
	g := myks.New(".")

	okOrFatal(g.ValidateRootDir(), "Root directory is not suitable for myks")
	okOrFatal(g.Init(asyncLevel, envAppMap), "Unable to initialize myks' globe")

	switch {
	case sync && render:
		okOrFatal(g.SyncAndRender(asyncLevel), "Unable to sync and render applications")
	case sync:
		okOrFatal(g.Sync(asyncLevel), "Unable to sync external sources")
	case render:
		okOrFatal(g.Render(asyncLevel), "Unable to render manifests")
	}

	// Cleaning up only if all environments and applications were processed
	if envAppMap == nil {
		okOrFatal(g.CleanupRenderedManifests(false), "Unable to cleanup rendered manifests")
	}
}
