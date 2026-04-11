package myks

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/rs/zerolog/log"
)

// InspectEnvironment holds inspection data for a single environment.
type InspectEnvironment struct {
	ID           string                  `json:"id"`
	Dir          string                  `json:"dir"`
	RenderedDir  string                  `json:"renderedDir"`
	ArgoDir      string                  `json:"argoDir"`
	ConfigFiles  []string                `json:"configFiles"`
	Applications []InspectEnvironmentApp `json:"applications"`
	DataValues   string                  `json:"dataValues,omitempty"`
}

// InspectEnvironmentApp is the brief app summary shown inside an InspectEnvironment.
type InspectEnvironmentApp struct {
	Name      string `json:"name"`
	Prototype string `json:"prototype"`
}

// InspectApplication groups all environment instances of an application by name.
type InspectApplication struct {
	Name      string               `json:"name"`
	Instances []InspectAppInstance `json:"instances"`
}

// InspectAppInstance holds inspection data for one (environment, application) pair.
type InspectAppInstance struct {
	EnvironmentID  string              `json:"environmentId"`
	EnvironmentDir string              `json:"environmentDir"`
	Prototype      string              `json:"prototype"`
	RenderedDir    string              `json:"renderedDir"`
	ServiceDir     string              `json:"serviceDir"`
	CommonFiles    InspectCommonFiles  `json:"commonFiles"`
	StepFiles      map[string][]string `json:"stepFiles"`
	DataValues     string              `json:"dataValues,omitempty"`
	Rendered       *InspectRendered    `json:"rendered,omitempty"`
}

// InspectCommonFiles holds the file paths shared across all rendering steps.
type InspectCommonFiles struct {
	ExtraYttPaths []string `json:"extraYttPaths"`
	YttDataFiles  []string `json:"yttDataFiles"`
}

// InspectRendered holds rendered artifacts read from the .myks/ service directory.
// Only populated when the --rendered flag is set.
type InspectRendered struct {
	VendirConfig string            `json:"vendirConfig,omitempty"`
	HelmValues   map[string]string `json:"helmValues,omitempty"`
	StepOutputs  map[string]string `json:"stepOutputs,omitempty"`
}

// InspectPrototype holds inspection data for a prototype and all applications that use it.
type InspectPrototype struct {
	Name   string                  `json:"name"`
	Dir    string                  `json:"dir"`
	Usages []InspectPrototypeUsage `json:"usages"`
}

// InspectPrototypeUsage describes one application that uses a prototype.
type InspectPrototypeUsage struct {
	AppName        string `json:"appName"`
	EnvironmentID  string `json:"environmentId"`
	EnvironmentDir string `json:"environmentDir"`
}

// InspectEnvironments returns inspection data for all initialized environments.
// If includeDataValues is true, each environment's final ytt data values are included.
func (g *Globe) InspectEnvironments(includeDataValues bool) ([]InspectEnvironment, error) {
	var result []InspectEnvironment

	for _, env := range g.environments {
		entry := InspectEnvironment{
			ID:          env.ID,
			Dir:         env.Dir,
			RenderedDir: filepath.Join(g.RenderedEnvsDir, env.ID),
			ArgoDir:     filepath.Join(g.RenderedArgoDir, env.ID),
			ConfigFiles: env.collectBySubpath(g.EnvironmentDataFileName),
		}

		for name, proto := range env.foundApplications {
			entry.Applications = append(entry.Applications, InspectEnvironmentApp{
				Name:      name,
				Prototype: proto,
			})
		}
		sort.Slice(entry.Applications, func(i, j int) bool {
			return entry.Applications[i].Name < entry.Applications[j].Name
		})

		if includeDataValues {
			envDataFiles := env.collectBySubpath(g.EnvironmentDataFileName)
			dataYaml, err := env.renderEnvData(envDataFiles)
			if err != nil {
				return nil, fmt.Errorf("rendering env data values for %s: %w", env.ID, err)
			}
			entry.DataValues = string(dataYaml)
		}

		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// InspectApplications returns inspection data for all initialized applications, grouped by name.
// If includeDataValues is true, each instance includes its final resolved ytt data values.
// If includeRendered is true, rendered artifacts from .myks/ are included if available.
func (g *Globe) InspectApplications(includeDataValues, includeRendered bool) ([]InspectApplication, error) {
	byName := map[string]*InspectApplication{}

	for _, env := range g.environments {
		for _, app := range env.Applications {
			instance, err := app.inspectInstance(includeDataValues, includeRendered)
			if err != nil {
				return nil, fmt.Errorf("inspecting %s/%s: %w", env.ID, app.Name, err)
			}

			if _, ok := byName[app.Name]; !ok {
				byName[app.Name] = &InspectApplication{Name: app.Name}
			}
			byName[app.Name].Instances = append(byName[app.Name].Instances, instance)
		}
	}

	result := make([]InspectApplication, 0, len(byName))
	for _, entry := range byName {
		sort.Slice(entry.Instances, func(i, j int) bool {
			return entry.Instances[i].EnvironmentID < entry.Instances[j].EnvironmentID
		})
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// InspectPrototypes returns inspection data for all prototypes found in use across
// all initialized environments, plus any prototypes present in the prototypes directory
// that are not currently used.
func (g *Globe) InspectPrototypes() ([]InspectPrototype, error) {
	byName := map[string]*InspectPrototype{}

	// Discover all prototype directories on disk.
	protoBaseDir := filepath.Join(g.RootDir, g.PrototypesDir)
	entries, err := os.ReadDir(protoBaseDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading prototypes directory %s: %w", protoBaseDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() && !isInternalDir(entry.Name()) {
			byName[entry.Name()] = &InspectPrototype{
				Name: entry.Name(),
				Dir:  filepath.Join(g.PrototypesDir, entry.Name()),
			}
		}
	}

	// Cross-reference with initialized environments to build usage list.
	for _, env := range g.environments {
		for _, app := range env.Applications {
			protoName := app.prototypeDirName()
			if _, ok := byName[protoName]; !ok {
				byName[protoName] = &InspectPrototype{
					Name: protoName,
					Dir:  filepath.Join(g.PrototypesDir, protoName),
				}
			}
			byName[protoName].Usages = append(byName[protoName].Usages, InspectPrototypeUsage{
				AppName:        app.Name,
				EnvironmentID:  env.ID,
				EnvironmentDir: env.Dir,
			})
		}
	}

	result := make([]InspectPrototype, 0, len(byName))
	for _, entry := range byName {
		sort.Slice(entry.Usages, func(i, j int) bool {
			if entry.Usages[i].EnvironmentID != entry.Usages[j].EnvironmentID {
				return entry.Usages[i].EnvironmentID < entry.Usages[j].EnvironmentID
			}
			return entry.Usages[i].AppName < entry.Usages[j].AppName
		})
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// isInternalDir returns true for underscore-prefixed directory names (e.g. _vendir).
func isInternalDir(name string) bool {
	return name != "" && name[0] == '_'
}

// inspectInstance builds an InspectAppInstance for this application.
func (a *Application) inspectInstance(includeDataValues, includeRendered bool) (InspectAppInstance, error) {
	stepFiles, err := a.inspectStepFiles()
	if err != nil {
		return InspectAppInstance{}, err
	}

	instance := InspectAppInstance{
		EnvironmentID:  a.e.ID,
		EnvironmentDir: a.e.Dir,
		Prototype:      a.Prototype,
		RenderedDir:    a.getDestinationDir(),
		ServiceDir:     a.expandServicePath(""),
		CommonFiles: InspectCommonFiles{
			ExtraYttPaths: slices.Clone(a.e.extraYttPaths),
			YttDataFiles:  slices.Clone(a.yttDataFiles),
		},
		StepFiles: stepFiles,
	}

	if includeDataValues {
		dataYaml, err := a.renderDataYaml(append(a.e.extraYttPaths, a.yttDataFiles...))
		if err != nil {
			return InspectAppInstance{}, fmt.Errorf("rendering data values: %w", err)
		}
		instance.DataValues = string(dataYaml)
	}

	if includeRendered {
		instance.Rendered = a.inspectRenderedArtifacts()
	}

	return instance, nil
}

// inspectStepFiles returns a map of pipeline step name to its source file paths.
// Only steps that have source files present on disk are included.
// Uses the same file-collection methods as the rendering steps themselves.
func (a *Application) inspectStepFiles() (map[string][]string, error) {
	result := map[string][]string{}

	vendirFiles, err := a.vendirSourceFiles()
	if err != nil {
		return nil, fmt.Errorf("collecting vendir source files: %w", err)
	}
	if len(vendirFiles) > 0 {
		result["sync-vendir"] = vendirFiles
	}

	if files := a.helmValuesSourceFiles(); len(files) > 0 {
		result["render-helm"] = files
	}

	yttFiles, err := a.yttSourceFiles()
	if err != nil {
		return nil, fmt.Errorf("collecting ytt source files: %w", err)
	}
	if len(yttFiles) > 0 {
		result["render-ytt"] = yttFiles
	}

	if files := a.yttPkgValuesSourceFiles(); len(files) > 0 {
		result["render-ytt-pkg"] = files
	}

	if files := a.globalYttSourceFiles(); len(files) > 0 {
		result["global-ytt"] = files
	}

	staticDirs, err := a.staticFilesSourceDirs()
	if err != nil {
		return nil, fmt.Errorf("collecting static files source dirs: %w", err)
	}
	if len(staticDirs) > 0 {
		result["static-files"] = staticDirs
	}

	argoCDFiles, err := a.argoCDAppSourceFiles()
	if err != nil {
		return nil, fmt.Errorf("collecting argocd source files: %w", err)
	}
	if len(argoCDFiles) > 0 {
		result["argocd"] = argoCDFiles
	}

	return result, nil
}

// warnIfUnexpectedReadErr logs a warning when err is non-nil and not an os.ErrNotExist
// error, so that real I/O problems (e.g. permission denied) are surfaced rather than
// silently treated as "artifact not present".
func (a *Application) warnIfUnexpectedReadErr(err error, path, msg string) {
	if !os.IsNotExist(err) {
		log.Warn().Err(err).Str("path", path).Msg(a.Msg("inspect", msg))
	}
}

// inspectRenderedArtifacts reads rendered artifacts from the .myks/ service directory.
// Returns nil if no artifacts are present. Does not trigger rendering.
func (a *Application) inspectRenderedArtifacts() *InspectRendered {
	rendered := &InspectRendered{}
	hasContent := false

	// Rendered vendir config
	vendirConfigPath := a.expandServicePath(a.cfg.VendirConfigFileName)
	if data, err := os.ReadFile(vendirConfigPath); err == nil {
		rendered.VendirConfig = string(data)
		hasContent = true
	} else {
		a.warnIfUnexpectedReadErr(err, vendirConfigPath, "Unable to read vendir config")
	}

	// Rendered helm values (skip unmerged_ files)
	helmDir := a.expandServicePath(a.cfg.HelmStepDirName)
	if entries, err := os.ReadDir(helmDir); err == nil {
		rendered.HelmValues = map[string]string{}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) >= 9 && name[:9] == "unmerged_" {
				continue
			}
			if filepath.Ext(name) != ".yaml" {
				continue
			}
			path := filepath.Join(helmDir, name)
			if data, err := os.ReadFile(path); err == nil {
				rendered.HelmValues[name] = string(data)
				hasContent = true
			} else {
				a.warnIfUnexpectedReadErr(err, path, "Unable to read helm values file")
			}
		}
		if len(rendered.HelmValues) == 0 {
			rendered.HelmValues = nil
		}
	} else {
		a.warnIfUnexpectedReadErr(err, helmDir, "Unable to read helm directory")
	}

	// Step outputs
	stepsDir := a.expandServicePath("steps")
	if entries, err := os.ReadDir(stepsDir); err == nil {
		rendered.StepOutputs = map[string]string{}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(stepsDir, entry.Name())
			if data, err := os.ReadFile(path); err == nil {
				rendered.StepOutputs[entry.Name()] = string(data)
				hasContent = true
			} else {
				a.warnIfUnexpectedReadErr(err, path, "Unable to read step output file")
			}
		}
		if len(rendered.StepOutputs) == 0 {
			rendered.StepOutputs = nil
		}
	} else {
		a.warnIfUnexpectedReadErr(err, stepsDir, "Unable to read steps directory")
	}

	if !hasContent {
		return nil
	}
	return rendered
}
