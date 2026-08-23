package myks

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"
	kcl "kcl-lang.io/kcl-go"
)

const (
	// kclModFileName is the KCL module manifest; its presence at the config root selects KCL mode.
	kclModFileName = "kcl.mod"
	// kclGeneratedAppDataFileName is the per-app data-values bridge file generated from the frozen tree.
	// It is the only ytt data file of a KCL-mode application.
	kclGeneratedAppDataFileName = "app-data.kcl-generated.ytt.yaml"
)

// kclGeneratedAppDataHeader turns the resolved app config into a ytt schema-extension document.
// A schema doc (not a plain data-values doc) is used so that keys unknown to the embedded
// data schema (e.g. prototype-specific `application` values) are accepted.
const kclGeneratedAppDataHeader = "#@data/values-schema\n#@overlay/match-child-defaults missing_ok=True\n---\n"

// kclTree is the frozen resolved tree produced by evaluating the KCL config root.
// It is the sole discovery mechanism for environments and applications in KCL mode.
type kclTree struct {
	// MyksSchemaVersion is the version of the myks schema the tree was built against.
	MyksSchemaVersion string `yaml:"myksSchemaVersion"`
	// Environments maps environment paths (relative to the environments base dir) to their resolved data.
	Environments map[string]kclEnvironmentData `yaml:"environments"`
}

// kclEnvironmentData is one environment entry of the frozen tree.
type kclEnvironmentData struct {
	// ID is the unique environment (cluster) identifier.
	ID string `yaml:"id"`
	// ArgoCD carries env-level ArgoCD settings.
	ArgoCD map[string]any `yaml:"argocd"`
	// Applications maps application names to their self-contained resolved config.
	Applications map[string]map[string]any `yaml:"applications"`
}

// isKclMode reports whether the repo opts into the KCL config layer (kcl.mod at the config root).
func (g *Globe) isKclMode() bool {
	ok, err := isExist(filepath.Join(g.RootDir, kclModFileName))
	if err != nil {
		log.Warn().Err(err).Msg(g.Msg("Unable to stat kcl.mod, falling back to legacy mode"))
		return false
	}
	return ok
}

// evalKclTree evaluates the KCL module at rootDir and parses the frozen resolved tree.
func evalKclTree(rootDir string) (*kclTree, error) {
	res, err := kcl.Run(rootDir)
	if err != nil {
		return nil, fmt.Errorf("evaluating KCL config at %s: %w", rootDir, err)
	}

	tree := &kclTree{}
	if err := yaml.Unmarshal([]byte(res.GetRawYamlResult()), tree); err != nil {
		return nil, fmt.Errorf("parsing KCL evaluation output: %w", err)
	}

	if tree.MyksSchemaVersion == "" {
		return nil, fmt.Errorf("KCL evaluation output is missing myksSchemaVersion")
	}
	// ponytail: no version compatibility assert yet; wired in with the published schema package (roadmap step 2)

	return tree, nil
}

// initFromKclTree initializes environments and applications from the frozen tree.
// No filesystem walk is performed: the tree is the sole discovery mechanism.
func (g *Globe) initFromKclTree(envSearchPathToAppMap EnvAppMap) error {
	tree, err := evalKclTree(g.RootDir)
	if err != nil {
		return err
	}
	log.Debug().Str("myksSchemaVersion", tree.MyksSchemaVersion).Int("environments", len(tree.Environments)).
		Msg(g.Msg("Initialized from KCL frozen tree"))

	filter := g.AddBaseDirToEnvAppMap(envSearchPathToAppMap)
	for _, envPath := range slices.Sorted(maps.Keys(tree.Environments)) {
		dir := filepath.Join(g.EnvironmentBaseDir, filepath.FromSlash(envPath))
		appNames, matched := matchEnvFilter(dir, filter)
		if !matched {
			continue
		}
		env, err := g.initKclEnvironment(dir, tree.Environments[envPath], appNames)
		if err != nil {
			return fmt.Errorf("initializing KCL environment %s: %w", envPath, err)
		}
		g.environments[dir] = env
	}

	return nil
}

// matchEnvFilter checks an environment dir against the CLI env/app selection.
// An empty filter matches everything. A dir matches a filter entry when it equals the
// entry's path or lies below it. Nil app names mean "all applications".
func matchEnvFilter(dir string, filter EnvAppMap) (appNames []string, matched bool) {
	if len(filter) == 0 {
		return nil, true
	}
	all := false
	for searchPath, names := range filter {
		if dir != searchPath && !strings.HasPrefix(dir, searchPath+string(filepath.Separator)) {
			continue
		}
		matched = true
		if len(names) == 0 {
			all = true
		}
		appNames = append(appNames, names...)
	}
	if !matched || all {
		return nil, matched
	}
	return unique(appNames), true
}

// initKclEnvironment builds a fully initialized Environment from one frozen-tree entry.
func (g *Globe) initKclEnvironment(dir string, envData kclEnvironmentData, appNames []string) (*Environment, error) {
	if envData.ID == "" {
		return nil, fmt.Errorf("environment entry is missing id")
	}

	env := &Environment{
		Dir:                     dir,
		ID:                      envData.ID,
		Applications:            []*Application{},
		g:                       g,
		cfg:                     &g.Config,
		extraYttPaths:           g.extraYttPaths,
		renderedDataLibFilePath: filepath.Join(g.RootDir, g.ServiceDirName, dir, g.RenderedEnvironmentDataLibFileName),
		foundApplications:       map[string]string{},
		kclMode:                 true,
	}

	// ArgoCD is opt-in per tree entry in KCL mode (unlike the legacy schema default of true).
	env.argoCDEnabled, _ = envData.ArgoCD["enabled"].(bool)

	envDataYaml, err := envData.dataValuesYaml()
	if err != nil {
		return nil, err
	}
	envDataLib, err := env.renderEnvDataLib(envDataYaml)
	if err != nil {
		return nil, fmt.Errorf("rendering environment data lib: %w", err)
	}
	if err := env.saveRenderedEnvDataLib(envDataLib); err != nil {
		return nil, fmt.Errorf("saving rendered environment data lib: %w", err)
	}

	for _, name := range slices.Sorted(maps.Keys(envData.Applications)) {
		appConfig := envData.Applications[name]
		proto := name
		if p, ok := appConfig["proto"].(string); ok && p != "" {
			proto = p
		}
		env.foundApplications[name] = proto

		bridgeFile := filepath.Join(g.RootDir, g.ServiceDirName, dir, g.AppsDir, name, kclGeneratedAppDataFileName)
		if err := writeKclAppDataFile(bridgeFile, appConfig); err != nil {
			return nil, fmt.Errorf("writing generated data values for app %s: %w", name, err)
		}
	}

	if err := env.initApplications(appNames); err != nil {
		return nil, fmt.Errorf("initializing applications: %w", err)
	}
	env.initialized = true

	return env, nil
}

// dataValuesYaml serializes the environment entry as ytt data values
// (feeds the env data lib consumed by templates via the myks ytt API).
func (d kclEnvironmentData) dataValuesYaml() ([]byte, error) {
	type appEntry struct {
		// Name of the application.
		Name string `yaml:"name"`
		// Proto is the application's prototype name.
		Proto string `yaml:"proto"`
	}
	data := struct {
		ArgoCD      map[string]any `yaml:"argocd,omitempty"`
		Environment struct {
			ID           string     `yaml:"id"`
			Applications []appEntry `yaml:"applications"`
		} `yaml:"environment"`
	}{ArgoCD: d.ArgoCD}
	data.Environment.ID = d.ID
	for _, name := range slices.Sorted(maps.Keys(d.Applications)) {
		proto, _ := d.Applications[name]["proto"].(string)
		if proto == "" {
			proto = name
		}
		data.Environment.Applications = append(data.Environment.Applications, appEntry{Name: name, Proto: proto})
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshalling environment data: %w", err)
	}
	return yamlBytes, nil
}

// writeKclAppDataFile writes the app's resolved config (minus the engine-only proto key)
// as a generated ytt data-values bridge file.
func writeKclAppDataFile(path string, appConfig map[string]any) error {
	values := maps.Clone(appConfig)
	delete(values, "proto")

	yamlBytes, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshalling app data values: %w", err)
	}
	return writeFile(path, append([]byte(kclGeneratedAppDataHeader), yamlBytes...))
}
