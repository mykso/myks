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
			env = strings.TrimSpace(env)
			if env == "" {
				continue
			}
			m[g.ResolveEnvIdentifier(env)] = nil
		}
		return m
	default: // len >= 2
		var appNames []string
		if args[1] != allEnvsToken {
			for app := range strings.SplitSeq(args[1], ",") {
				app = strings.TrimSpace(app)
				if app == "" {
					continue
				}
				appNames = append(appNames, app)
			}
		}
		m := make(myks.EnvAppMap)
		g := getGlobe()
		if args[0] == allEnvsToken {
			m[g.EnvironmentBaseDir] = appNames
		} else {
			for env := range strings.SplitSeq(args[0], ",") {
				env = strings.TrimSpace(env)
				if env == "" {
					continue
				}
				m[g.ResolveEnvIdentifier(env)] = appNames
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

const inspectEnvsUsageTemplate = `Usage:
  {{.CommandPath}} [env-selector] [flags]

Arguments:
  env-selector    (Optional) Comma-separated list of environments or ALL
                  When omitted, all environments are processed.
                  ALL will also process all environments.
                  Examples: ALL
                            prod,stage,dev
                            prod/region1,stage/region1
                            dev

{{if .HasAvailableFlags}}Flags:
{{.Flags.FlagUsages | trimTrailingWhitespaces}}{{end}}
Examples:
  # Inspect all environments
  {{.CommandPath}}

  # Inspect specific environments
  {{.CommandPath}} prod,stage

  # Inspect all environments explicitly
  {{.CommandPath}} ALL
`

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
	cmd.SetUsageTemplate(inspectEnvsUsageTemplate)
	return cmd
}

// ── inspect apps ──────────────────────────────────────────────────────────────

const inspectAppsUsageTemplate = `Usage:
  {{.CommandPath}} [env-selector [app-selector]] [flags]

Arguments:
  env-selector    (Optional) Comma-separated list of environments or ALL
                  When omitted, all environments are processed.
                  ALL will also process all environments.
                  Examples: ALL
                            prod,stage,dev
                            prod/region1,stage/region1

  app-selector    (Optional) Comma-separated list of applications or ALL
                  When omitted, all applications are processed.
                  ALL will also process all applications.
                  Examples: ALL
                            app1,app2

{{if .HasAvailableFlags}}Flags:
{{.Flags.FlagUsages | trimTrailingWhitespaces}}{{end}}
Examples:
  # Inspect all apps in all environments
  {{.CommandPath}}

  # Inspect all apps in production and staging
  {{.CommandPath}} prod,stage ALL

  # Inspect specific apps in all environments
  {{.CommandPath}} ALL app1,app2

  # Inspect specific apps in specific environments
  {{.CommandPath}} prod,stage app1,app2

  # Show which data-values, schema, helm or ytt-pkg values file sets each final value
  {{.CommandPath}} prod app1 --provenance

  # Also list the values that come from the myks built-in defaults
  {{.CommandPath}} prod app1 --provenance=all

  # Fail when a data-values, helm or ytt-pkg values file sets a value to the one it already inherits (for CI)
  {{.CommandPath}} --redundant
`

// --provenance modes: the text output lists only the values a data-values or schema file sets,
// or all of them. JSON always lists all, so a consumer filters on an empty file itself.
const (
	provenanceSet = "set"
	provenanceAll = "all"
)

func newInspectAppsCmd() *cobra.Command {
	var opts myks.InspectAppsOptions
	var provenance string

	cmd := &cobra.Command{
		Use:               "apps [env-selector [app-selector]]",
		Short:             "Inspect applications",
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: shellCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch provenance {
			case "", provenanceSet, provenanceAll:
			default:
				return fmt.Errorf("unsupported --provenance value %q: want %q or %q", provenance, provenanceSet, provenanceAll)
			}
			opts.Provenance = provenance != ""

			g := getGlobe()
			if err := g.ValidateRootDir(); err != nil {
				return fmt.Errorf("root directory is not suitable for myks: %w", err)
			}

			envAppMap := parseInspectEnvAppMap(args)
			if err := g.Init(asyncLevel, envAppMap); err != nil {
				return fmt.Errorf("unable to initialize: %w", err)
			}

			apps, err := g.InspectApplications(asyncLevel, opts)
			if err != nil {
				return fmt.Errorf("inspect failed: %w", err)
			}

			if err := printOutput(cmd, apps, func() {
				printInspectApps(apps, provenance == provenanceAll)
			}); err != nil {
				return err
			}

			if n := countRedundantOverrides(apps); n > 0 {
				// A finding, not a misuse: the usage text would only bury it.
				cmd.SilenceUsage = true
				return fmt.Errorf("found %d redundant overrides", n)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&opts.DataValues, "data-values", false, "include final resolved ytt data values")
	cmd.Flags().BoolVar(&opts.Rendered, "rendered", false, "include rendered artifacts from .myks/ if available")
	cmd.Flags().StringVar(&provenance, "provenance", "", `include the file that sets each final data value and helm or ytt-pkg value; "all" also lists the myks built-in data value defaults in the text output`)
	cmd.Flags().Lookup("provenance").NoOptDefVal = provenanceSet
	cmd.Flags().BoolVar(&opts.Redundant, "redundant", false, "include data values and helm or ytt-pkg values a file sets to the value it already inherits; exit non-zero if any")
	cmd.SetUsageTemplate(inspectAppsUsageTemplate)
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
	fmt.Printf("%s %s\n", aurora.Bold(aurora.Cyan("Environment:")), aurora.Bold(aurora.Cyan(env.ID)))
	fmt.Printf("  %s %s\n", aurora.Bold("Dir:     "), aurora.Faint(env.Dir))
	fmt.Printf("  %s %s\n", aurora.Bold("Rendered:"), aurora.Faint(env.RenderedDir))
	fmt.Printf("  %s %s\n", aurora.Bold("ArgoCD:  "), aurora.Faint(env.ArgoDir))
	if len(env.ConfigFiles) > 0 {
		fmt.Printf("  %s\n", aurora.Yellow("Config:"))
		for _, f := range env.ConfigFiles {
			fmt.Printf("    - %s\n", aurora.Faint(f))
		}
	}
	if len(env.Applications) > 0 {
		fmt.Printf("  %s\n", aurora.Yellow("Applications:"))
		for _, app := range env.Applications {
			fmt.Printf("    %s %s %s\n", aurora.Green(fmt.Sprintf("%-30s", app.Name)), aurora.Faint("prototype:"), aurora.Green(app.Prototype))
		}
	}
	if env.DataValues != "" {
		fmt.Printf("  %s\n", aurora.Yellow("Data Values:"))
		for _, line := range strings.Split(strings.TrimRight(env.DataValues, "\n"), "\n") {
			fmt.Printf("    %s\n", line)
		}
	}
}

// knownStepOrder defines the display order for pipeline steps.
var knownStepOrder = []string{"sync-vendir", "render-helm", "render-ytt", "render-ytt-pkg", "global-ytt", "static-files", "argocd"}

func printInspectApps(apps []myks.InspectApplication, withDefaults bool) {
	for i := range apps {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s %s\n", aurora.Bold(aurora.Cyan("Application:")), aurora.Bold(aurora.Cyan(apps[i].Name)))
		for j := range apps[i].Instances {
			fmt.Println()
			printInspectAppInstance(&apps[i].Instances[j], withDefaults)
		}
	}
}

func printInspectAppInstance(inst *myks.InspectAppInstance, withDefaults bool) {
	fmt.Printf("  %s %s %s\n", aurora.Bold(aurora.Cyan("Environment:")), aurora.Cyan(inst.EnvironmentID), aurora.Faint("("+inst.EnvironmentDir+")"))
	fmt.Printf("  %s %s\n", aurora.Bold("Prototype: "), aurora.Green(inst.Prototype))
	fmt.Printf("  %s %s\n", aurora.Bold("Rendered:  "), aurora.Faint(inst.RenderedDir))
	fmt.Printf("  %s %s\n", aurora.Bold("Service:   "), aurora.Faint(inst.ServiceDir))

	fmt.Println()
	fmt.Printf("  %s\n", aurora.Yellow("Common Files:"))
	for _, f := range inst.CommonFiles.ExtraYttPaths {
		fmt.Printf("    - %s\n", aurora.Faint(f))
	}
	for _, f := range inst.CommonFiles.YttDataFiles {
		fmt.Printf("    - %s\n", aurora.Faint(f))
	}

	if len(inst.StepFiles) > 0 {
		printInspectStepFiles(inst.StepFiles)
	}

	if inst.DataValues != "" {
		fmt.Println()
		fmt.Printf("  %s\n", aurora.Yellow("Data Values:"))
		for _, line := range strings.Split(strings.TrimRight(inst.DataValues, "\n"), "\n") {
			fmt.Printf("    %s\n", line)
		}
	}

	if inst.Rendered != nil {
		printInspectRendered(inst.Rendered)
	}

	sources := inst.Provenance
	if !withDefaults {
		sources = slices.DeleteFunc(slices.Clone(sources), func(s myks.InspectValueSource) bool { return s.File == "" })
	}
	printInspectProvenance("Provenance:", sources)
	printInspectRedundant("Redundant Overrides:", inst.Redundant)
	for _, chart := range slices.Sorted(maps.Keys(inst.HelmProvenance)) {
		printInspectProvenance("Helm Provenance ("+chart+"):", inst.HelmProvenance[chart])
	}
	for _, chart := range slices.Sorted(maps.Keys(inst.HelmRedundant)) {
		printInspectRedundant("Helm Redundant Overrides ("+chart+"):", inst.HelmRedundant[chart])
	}
	for _, pkg := range slices.Sorted(maps.Keys(inst.YttPkgProvenance)) {
		printInspectProvenance("ytt-pkg Provenance ("+pkg+"):", inst.YttPkgProvenance[pkg])
	}
	for _, pkg := range slices.Sorted(maps.Keys(inst.YttPkgRedundant)) {
		printInspectRedundant("ytt-pkg Redundant Overrides ("+pkg+"):", inst.YttPkgRedundant[pkg])
	}
}

func printInspectProvenance(title string, sources []myks.InspectValueSource) {
	if len(sources) == 0 {
		return
	}
	fmt.Println()
	fmt.Printf("  %s\n", aurora.Yellow(title))
	for _, s := range sources {
		file := s.File
		if file == "" {
			file = "(built-in defaults)"
		}
		fmt.Printf("    %s %s\n", aurora.Green(s.Path), aurora.Faint(file))
	}
}

func printInspectRedundant(title string, redundant []myks.InspectRedundantOverride) {
	if len(redundant) == 0 {
		return
	}
	fmt.Println()
	fmt.Printf("  %s\n", aurora.Yellow(title))
	for _, r := range redundant {
		value, _ := json.Marshal(r.Value)
		fmt.Printf("    %s = %s %s\n", aurora.Red(r.Path), value, aurora.Faint(r.File))
	}
}

func countRedundantOverrides(apps []myks.InspectApplication) int {
	n := 0
	for _, app := range apps {
		for i := range app.Instances {
			inst := &app.Instances[i]
			n += len(inst.Redundant)
			for _, r := range inst.HelmRedundant {
				n += len(r)
			}
			for _, r := range inst.YttPkgRedundant {
				n += len(r)
			}
		}
	}
	return n
}

func printInspectStepFiles(stepFiles map[string][]string) {
	fmt.Println()
	fmt.Printf("  %s\n", aurora.Yellow("Step Files:"))
	printed := map[string]bool{}
	for _, step := range knownStepOrder {
		files, ok := stepFiles[step]
		if !ok {
			continue
		}
		printed[step] = true
		fmt.Printf("    %s:\n", aurora.Green(step))
		for _, f := range files {
			fmt.Printf("      - %s\n", aurora.Faint(f))
		}
	}
	// Print any steps not in the known order, sorted for deterministic output.
	for _, step := range slices.Sorted(maps.Keys(stepFiles)) {
		if printed[step] {
			continue
		}
		fmt.Printf("    %s:\n", aurora.Green(step))
		for _, f := range stepFiles[step] {
			fmt.Printf("      - %s\n", aurora.Faint(f))
		}
	}
}

func printInspectRendered(r *myks.InspectRendered) {
	fmt.Println()
	fmt.Printf("  %s\n", aurora.Yellow("Rendered Artifacts:"))
	if r.VendirConfig != "" {
		fmt.Printf("    vendir.yaml: %s\n", aurora.Faint("(present)"))
	}
	if len(r.HelmValues) > 0 {
		fmt.Printf("    %s\n", aurora.Bold("Helm values:"))
		for _, name := range slices.Sorted(maps.Keys(r.HelmValues)) {
			fmt.Printf("      - %s\n", aurora.Green(name))
		}
	}
	if len(r.StepOutputs) > 0 {
		fmt.Printf("    %s\n", aurora.Bold("Step outputs:"))
		for _, name := range slices.Sorted(maps.Keys(r.StepOutputs)) {
			fmt.Printf("      - %s\n", aurora.Green(name))
		}
	}
}

func printInspectPrototypes(protos []myks.InspectPrototype) {
	for i, proto := range protos {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s %s\n", aurora.Bold(aurora.Cyan("Prototype:")), aurora.Bold(aurora.Cyan(proto.Name)))
		fmt.Printf("  %s %s\n", aurora.Bold("Dir:"), aurora.Faint(proto.Dir))
		if len(proto.Usages) > 0 {
			fmt.Printf("  %s\n", aurora.Yellow("Used by:"))
			for _, u := range proto.Usages {
				fmt.Printf("    %s %s %s %s\n", aurora.Green(fmt.Sprintf("%-30s", u.AppName)), aurora.Faint("in"), aurora.Cyan(u.EnvironmentID), aurora.Faint("("+u.EnvironmentDir+")"))
			}
		} else {
			fmt.Printf("  Used by: %s\n", aurora.Faint("(none)"))
		}
	}
}
