package cmd

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	aurora "github.com/logrusorgru/aurora/v4"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

const (
	inspectOutputText = "text"
	inspectOutputJSON = "json"
)

func newInspectCmd() *cobra.Command {
	inspectCmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect environments, applications, and prototypes",
		Long: `Display structural information about environments, applications, and prototypes.

The inspect command does not run the full sync/render pipeline (vendir, Helm,
ytt, kbld, ArgoCD). It initializes environments and applications to resolve
configuration, data values, and file paths, which writes intermediate files to
the .myks/ service directory.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return validateOutputFormat(cmd)
		},
	}

	inspectCmd.PersistentFlags().StringP("output", "o", inspectOutputText, `output format: "text" or "json"`)
	inspectCmd.AddCommand(newInspectEnvsCmd())
	inspectCmd.AddCommand(newInspectAppsCmd())
	inspectCmd.AddCommand(newInspectPrototypesCmd())
	return inspectCmd
}

// parseInspectEnvAppMap parses 0–2 positional arguments into an EnvAppMap,
// following the same conventions as the render command.
func parseInspectEnvAppMap(args []string) myks.EnvAppMap {
	switch len(args) {
	case 0:
		return nil // all envs, all apps
	case 1:
		if args[0] == allEnvsToken {
			return nil
		}
		m := make(myks.EnvAppMap)
		g := getGlobe()
		for env := range strings.SplitSeq(args[0], ",") {
			m[g.ResolveEnvIdentifier(strings.TrimSpace(env))] = nil
		}
		return m
	default: // len >= 2
		var appNames []string
		if args[1] != allEnvsToken {
			for app := range strings.SplitSeq(args[1], ",") {
				appNames = append(appNames, strings.TrimSpace(app))
			}
		}
		m := make(myks.EnvAppMap)
		g := getGlobe()
		if args[0] == allEnvsToken {
			m[g.EnvironmentBaseDir] = appNames
		} else {
			for env := range strings.SplitSeq(args[0], ",") {
				m[g.ResolveEnvIdentifier(strings.TrimSpace(env))] = appNames
			}
		}
		return m
	}
}

// validateOutputFormat returns an error if the --output flag has an unsupported value.
func validateOutputFormat(cmd *cobra.Command) error {
	switch outputFormat(cmd) {
	case inspectOutputText, inspectOutputJSON:
		return nil
	default:
		return fmt.Errorf("unsupported output format %q: must be %q or %q", outputFormat(cmd), inspectOutputText, inspectOutputJSON)
	}
}

// outputFormat returns the --output flag value from the nearest parent that has it.
func outputFormat(cmd *cobra.Command) string {
	for c := cmd; c != nil; c = c.Parent() {
		if f := c.Flags().Lookup("output"); f != nil {
			return f.Value.String()
		}
		if f := c.PersistentFlags().Lookup("output"); f != nil {
			return f.Value.String()
		}
	}
	return inspectOutputText
}

// ── inspect envs ──────────────────────────────────────────────────────────────

func newInspectEnvsCmd() *cobra.Command {
	var dataValues bool

	cmd := &cobra.Command{
		Use:   "envs [env-selector]",
		Short: "Inspect environments",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return getEnvCompletions(getGlobe()), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			g := getGlobe()
			if err := g.ValidateRootDir(); err != nil {
				return fmt.Errorf("root directory is not suitable for myks: %w", err)
			}

			envAppMap := parseInspectEnvAppMap(args)
			if err := g.Init(asyncLevel, envAppMap); err != nil {
				return fmt.Errorf("unable to initialize: %w", err)
			}

			envs, err := g.InspectEnvironments(dataValues)
			if err != nil {
				return fmt.Errorf("inspect failed: %w", err)
			}

			return printOutput(cmd, envs, func() {
				printInspectEnvs(envs)
			})
		},
	}

	cmd.Flags().BoolVar(&dataValues, "data-values", false, "include final resolved ytt data values")
	return cmd
}

// ── inspect apps ──────────────────────────────────────────────────────────────

func newInspectAppsCmd() *cobra.Command {
	var dataValues bool
	var rendered bool

	cmd := &cobra.Command{
		Use:               "apps [env-selector [app-selector]]",
		Short:             "Inspect applications",
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: shellCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			g := getGlobe()
			if err := g.ValidateRootDir(); err != nil {
				return fmt.Errorf("root directory is not suitable for myks: %w", err)
			}

			envAppMap := parseInspectEnvAppMap(args)
			if err := g.Init(asyncLevel, envAppMap); err != nil {
				return fmt.Errorf("unable to initialize: %w", err)
			}

			apps, err := g.InspectApplications(dataValues, rendered)
			if err != nil {
				return fmt.Errorf("inspect failed: %w", err)
			}

			return printOutput(cmd, apps, func() {
				printInspectApps(apps)
			})
		},
	}

	cmd.Flags().BoolVar(&dataValues, "data-values", false, "include final resolved ytt data values")
	cmd.Flags().BoolVar(&rendered, "rendered", false, "include rendered artifacts from .myks/ if available")
	return cmd
}

// ── inspect prototypes ────────────────────────────────────────────────────────

func newInspectPrototypesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prototypes [prototype-name]",
		Short: "Inspect prototypes",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			g := getGlobe()
			if err := g.ValidateRootDir(); err != nil {
				return fmt.Errorf("root directory is not suitable for myks: %w", err)
			}

			// To build prototype usage, we need all environments initialized.
			if err := g.Init(asyncLevel, nil); err != nil {
				return fmt.Errorf("unable to initialize: %w", err)
			}

			protos, err := g.InspectPrototypes()
			if err != nil {
				return fmt.Errorf("inspect failed: %w", err)
			}

			// Optional filter by name argument.
			if len(args) == 1 {
				name := args[0]
				filtered := protos[:0]
				for _, p := range protos {
					if p.Name == name {
						filtered = append(filtered, p)
					}
				}
				protos = filtered
			}

			return printOutput(cmd, protos, func() {
				printInspectPrototypes(protos)
			})
		},
	}

	return cmd
}

// ── output helpers ────────────────────────────────────────────────────────────

// printOutput writes either JSON or human-readable text, depending on --output.
func printOutput(cmd *cobra.Command, data any, textPrinter func()) error {
	switch outputFormat(cmd) {
	case inspectOutputJSON:
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("marshalling to JSON: %w", err)
		}
		fmt.Println(string(out))
	case inspectOutputText:
		textPrinter()
	default:
		return fmt.Errorf("unsupported output format: %q", outputFormat(cmd))
	}
	return nil
}

func printInspectEnvs(envs []myks.InspectEnvironment) {
	for i := range envs {
		if i > 0 {
			fmt.Println()
		}
		printInspectEnv(&envs[i])
	}
}

func printInspectEnv(env *myks.InspectEnvironment) {
	fmt.Printf("%s %s\n", aurora.Bold("Environment:"), aurora.Bold(env.ID))
	fmt.Printf("  Dir:      %s\n", env.Dir)
	fmt.Printf("  Rendered: %s\n", env.RenderedDir)
	fmt.Printf("  ArgoCD:   %s\n", env.ArgoDir)
	if len(env.ConfigFiles) > 0 {
		fmt.Println("  Config:")
		for _, f := range env.ConfigFiles {
			fmt.Printf("    - %s\n", f)
		}
	}
	if len(env.Applications) > 0 {
		fmt.Println("  Applications:")
		for _, app := range env.Applications {
			fmt.Printf("    %-30s prototype: %s\n", app.Name, app.Prototype)
		}
	}
	if env.DataValues != "" {
		fmt.Println("  Data Values:")
		for _, line := range strings.Split(strings.TrimRight(env.DataValues, "\n"), "\n") {
			fmt.Printf("    %s\n", line)
		}
	}
}

// knownStepOrder defines the display order for pipeline steps.
var knownStepOrder = []string{"sync-vendir", "render-helm", "render-ytt", "render-ytt-pkg", "global-ytt", "static-files", "argocd"}

func printInspectApps(apps []myks.InspectApplication) {
	for i := range apps {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s %s\n", aurora.Bold("Application:"), aurora.Bold(apps[i].Name))
		for j := range apps[i].Instances {
			fmt.Println()
			printInspectAppInstance(&apps[i].Instances[j])
		}
	}
}

func printInspectAppInstance(inst *myks.InspectAppInstance) {
	fmt.Printf("  %s %s (%s)\n", aurora.Bold("Environment:"), inst.EnvironmentID, inst.EnvironmentDir)
	fmt.Printf("  Prototype:  %s\n", inst.Prototype)
	fmt.Printf("  Rendered:   %s\n", inst.RenderedDir)
	fmt.Printf("  Service:    %s\n", inst.ServiceDir)

	fmt.Println()
	fmt.Println("  Common Files:")
	for _, f := range inst.CommonFiles.ExtraYttPaths {
		fmt.Printf("    - %s\n", f)
	}
	for _, f := range inst.CommonFiles.YttDataFiles {
		fmt.Printf("    - %s\n", f)
	}

	if len(inst.StepFiles) > 0 {
		printInspectStepFiles(inst.StepFiles)
	}

	if inst.DataValues != "" {
		fmt.Println()
		fmt.Println("  Data Values:")
		for _, line := range strings.Split(strings.TrimRight(inst.DataValues, "\n"), "\n") {
			fmt.Printf("    %s\n", line)
		}
	}

	if inst.Rendered != nil {
		printInspectRendered(inst.Rendered)
	}
}

func printInspectStepFiles(stepFiles map[string][]string) {
	fmt.Println()
	fmt.Println("  Step Files:")
	printed := map[string]bool{}
	for _, step := range knownStepOrder {
		files, ok := stepFiles[step]
		if !ok {
			continue
		}
		printed[step] = true
		fmt.Printf("    %s:\n", step)
		for _, f := range files {
			fmt.Printf("      - %s\n", f)
		}
	}
	// Print any steps not in the known order, sorted for deterministic output.
	for _, step := range slices.Sorted(maps.Keys(stepFiles)) {
		if printed[step] {
			continue
		}
		fmt.Printf("    %s:\n", step)
		for _, f := range stepFiles[step] {
			fmt.Printf("      - %s\n", f)
		}
	}
}

func printInspectRendered(r *myks.InspectRendered) {
	fmt.Println()
	fmt.Println("  Rendered Artifacts:")
	if r.VendirConfig != "" {
		fmt.Println("    vendir.yaml: (present)")
	}
	if len(r.HelmValues) > 0 {
		fmt.Println("    Helm values:")
		for _, name := range slices.Sorted(maps.Keys(r.HelmValues)) {
			fmt.Printf("      - %s\n", name)
		}
	}
	if len(r.StepOutputs) > 0 {
		fmt.Println("    Step outputs:")
		for _, name := range slices.Sorted(maps.Keys(r.StepOutputs)) {
			fmt.Printf("      - %s\n", name)
		}
	}
}

func printInspectPrototypes(protos []myks.InspectPrototype) {
	for i, proto := range protos {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s %s\n", aurora.Bold("Prototype:"), aurora.Bold(proto.Name))
		fmt.Printf("  Dir: %s\n", proto.Dir)
		if len(proto.Usages) > 0 {
			fmt.Println("  Used by:")
			for _, u := range proto.Usages {
				fmt.Printf("    %-30s in %s (%s)\n", u.AppName, u.EnvironmentID, u.EnvironmentDir)
			}
		} else {
			fmt.Println("  Used by: (none)")
		}
	}
}
