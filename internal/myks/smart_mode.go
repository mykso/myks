package myks

import (
	"errors"
	"maps"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type SmartMode struct {
	g *Globe
}

func NewSmartMode(g *Globe) *SmartMode {
	return &SmartMode{g: g}
}

func (sm *SmartMode) DetectChangedEnvsAndApps(baseRevision string) (EnvAppMap, error) {
	if !sm.g.WithGit {
		return nil, errors.New("git is unavailable")
	}

	// envAppMap is built later by calling sm.runSmartMode
	_ = sm.g.collectEnvironments(nil)

	err := process(0, maps.Values(sm.g.environments), func(env *Environment) error {
		return env.initEnvData()
	})
	if err != nil {
		log.Err(err).Msg(sm.g.Msg("Failed to collect environments"))
		return nil, err
	}

	changedFiles, err := GetChangedFilesGit(baseRevision, sm.g.Metrics)
	if err != nil {
		log.Err(err).Msg(sm.g.Msg("Failed to get diff"))
		return nil, err
	}
	log.Trace().Interface("changedFiles", changedFiles).Msg(sm.g.Msg("Detected changes"))
	envAppMap := sm.runSmartMode(changedFiles)
	for env, apps := range envAppMap {
		log.Debug().Str("env", env).Strs("apps", apps).Msg(sm.g.Msg("Detected changes"))
	}

	return envAppMap, nil
}

// find apps that are missing from rendered folder
func (sm *SmartMode) missingApplications() (EnvAppMap, error) {
	envsWithMissingApps := EnvAppMap{}
	for path, e := range sm.g.environments {
		missing, err := e.missingApplications()
		if err != nil {
			log.Err(err).Str("env", path).Msg(sm.g.Msg("Failed to get missing applications"))
			return nil, err
		}
		if len(missing) > 0 {
			envsWithMissingApps[path] = missing
		}
	}

	return envsWithMissingApps, nil
}

func (sm *SmartMode) buildSmartModeExprMap() map[string][]*regexp.Regexp {
	e := func(sample string) *regexp.Regexp {
		return regexp.MustCompile("^" + sm.g.Config.GitPathPrefix + sample + "$")
	}

	globToRegexp := func(glob string) string {
		r := glob
		r = strings.ReplaceAll(r, ".", "\\.")
		r = strings.ReplaceAll(r, "*", ".*")
		return r
	}

	// Subdirectories of apps and prototypes are named after plugins
	plugins := []string{
		sm.g.Config.ArgoCDDataDirName,
		sm.g.Config.HelmStepDirName,
		sm.g.Config.StaticFilesDirName,
		sm.g.Config.VendirStepDirName,
		sm.g.Config.YttPkgStepDirName,
		sm.g.Config.YttStepDirName,
		"lib",
	}
	pluginsPattern := "(?:" + strings.Join(plugins, "|") + ")"

	return map[string][]*regexp.Regexp{
		"global": {
			e(sm.g.Config.YttLibraryDirName + "/.*"),
			e(sm.g.Config.PrototypesDir + "/_vendir/.*"),
		},
		"env": {
			e("(" + sm.g.Config.EnvironmentBaseDir + ".*)/" + sm.g.Config.EnvsDir + "/" + sm.g.Config.YttStepDirName + "/.*"),
			e("(" + sm.g.Config.EnvironmentBaseDir + ".*)/" + sm.g.Config.EnvsDir + "/" + sm.g.Config.ArgoCDDataDirName + "/.*"),
			e("(" + sm.g.Config.EnvironmentBaseDir + ".*)/" + globToRegexp(sm.g.Config.EnvironmentDataFileName)),
		},
		"prototype": {
			e(sm.g.Config.PrototypesDir + "/(.+)/" + pluginsPattern + "/.*"),
			e(sm.g.Config.PrototypesDir + "/(.+)/" + globToRegexp(sm.g.Config.ApplicationDataFileName)),
		},
		"env-prototype": {
			e("(" + sm.g.Config.EnvironmentBaseDir + ".*)/" + sm.g.Config.PrototypeOverrideDir + "/([^/]+)/" + pluginsPattern + "/.*"),
			e("(" + sm.g.Config.EnvironmentBaseDir + ".*)/" + sm.g.Config.PrototypeOverrideDir + "/([^/]+)/" + globToRegexp(sm.g.Config.ApplicationDataFileName)),
		},
		"app": {
			e("(" + sm.g.Config.EnvironmentBaseDir + ".*)/" + sm.g.Config.AppsDir + "/([^/]+)/" + pluginsPattern + "/.*"),
			e("(" + sm.g.Config.EnvironmentBaseDir + ".*)/" + sm.g.Config.AppsDir + "/([^/]+)/" + globToRegexp(sm.g.Config.ApplicationDataFileName)),
		},
		"rendered-app": {
			e(sm.g.Config.RenderedEnvsDir + "/([^/]+)/([^/]+)/.*"),
			e(sm.g.Config.RenderedArgoDir + "/([^/]+)/app-([^/]+)\\.yaml"),
		},
	}
}

func extractMatches(g *Globe, exprs []*regexp.Regexp, path string) []string {
	for _, expr := range exprs {
		submatches := expr.FindStringSubmatch(path)
		log.Trace().
			Str("pattern", expr.String()).
			Str("path", path).
			Bool("matched", submatches != nil).
			Msg(g.Msg("Extracting submatches"))

		if submatches != nil {
			return submatches[1:]
		}
	}
	return nil
}

func (sm *SmartMode) processChangedFile(path string, exprMap map[string][]*regexp.Regexp, envAppMap EnvAppMap, changedEnvs *[]string, changedPrototypes *[]string) bool {
	// Check if the global configuration has changed
	if extractMatches(sm.g, exprMap["global"], path) != nil {
		return true // indicates global change
	}

	// If env has changed
	if envMatch := extractMatches(sm.g, exprMap["env"], path); envMatch != nil {
		envPath := sm.g.AddBaseDirToEnvPath(envMatch[0])
		*changedEnvs = append(*changedEnvs, envPath)
		return false
	}

	// If prototype has changed
	if protoMatch := extractMatches(sm.g, exprMap["prototype"], path); protoMatch != nil {
		*changedPrototypes = append(*changedPrototypes, protoMatch[0])
		return false
	}

	// If environment-specific prototype has changed
	if envProtoMatch := extractMatches(sm.g, exprMap["env-prototype"], path); envProtoMatch != nil {
		envPath := sm.g.AddBaseDirToEnvPath(envProtoMatch[0])
		prototypeName := envProtoMatch[1]
		for env, apps := range sm.findPrototypeUsage([]string{prototypeName}, envPath) {
			envAppMap[env] = append(envAppMap[env], apps...)
		}
		return false
	}

	// If app has changed
	if appMatch := extractMatches(sm.g, exprMap["app"], path); appMatch != nil {
		envPath := sm.g.AddBaseDirToEnvPath(appMatch[0])
		envAppMap[envPath] = append(envAppMap[envPath], appMatch[1])
		return false
	}

	// If rendered app has changed
	if appMatch := extractMatches(sm.g, exprMap["rendered-app"], path); appMatch != nil {
		env, err := sm.g.getEnvByID(appMatch[0])
		if err != nil {
			log.Err(err).Str("envID", appMatch[0]).Msg(sm.g.Msg("Failed to get environment by ID"))
			return false
		}
		envPath := sm.g.AddBaseDirToEnvPath(env.Dir)
		envAppMap[envPath] = append(envAppMap[envPath], appMatch[1])
		return false
	}

	return false
}

func (sm *SmartMode) runSmartMode(changedFiles ChangedFiles) EnvAppMap {
	exprMap := sm.buildSmartModeExprMap()

	// Here we start collecting changed environments and applications,
	// starting with those that are missed from the rendered directory.
	envAppMap, err := sm.missingApplications()
	if err != nil {
		log.Err(err).Msg(sm.g.Msg("Failed to get missing applications"))
	}

	changedEnvs := []string{}
	changedPrototypes := []string{}

	for path := range changedFiles {
		if sm.processChangedFile(path, exprMap, envAppMap, &changedEnvs, &changedPrototypes) {
			return EnvAppMap{sm.g.Config.EnvironmentBaseDir: nil}
		}
	}

	for env, apps := range sm.findPrototypeUsage(changedPrototypes, "") {
		envAppMap[env] = append(envAppMap[env], apps...)
	}

	// If env has changed, all apps in that env are affected
	for _, env := range changedEnvs {
		envAppMap[env] = nil
	}

	for env, apps := range envAppMap {
		if apps != nil {
			envAppMap[env] = unique(apps)
		}
	}

	sm.filterMissingEnvsApps(envAppMap)
	return envAppMap
}

func (sm *SmartMode) filterMissingEnvsApps(envAppMap EnvAppMap) {
	// Remove environments and applications that are not found in the filesystem
	for env, apps := range envAppMap {
		// env can be an exact path of an environment or one of parent directories
		matchedEnvs := sm.getEnvironmentsUnderRoot(env)
		if len(matchedEnvs) == 0 {
			delete(envAppMap, env)
			continue
		}
		for _, app := range apps {
			// env can be absent in g.environments if it is a parent directory of an environment
			// in this case we can't easily check if app is present in env
			// TODO: implement smarter lookup logic instead
			if _, ok := sm.g.environments[env]; !ok {
				continue
			}
			if _, ok := sm.g.environments[env].foundApplications[app]; !ok {
				envAppMap[env] = filterSlice(envAppMap[env], func(s string) bool { return s != app })
			}
		}
	}
}

func (sm *SmartMode) findPrototypeUsage(prototypes []string, envRoot string) EnvAppMap {
	envAppMap := EnvAppMap{}
	if envRoot == "" {
		envRoot = sm.g.Config.EnvironmentBaseDir
	}

		matchedEnvs := sm.getEnvironmentsUnderRoot(envRoot)

	for _, prototype := range prototypes {
		for _, envPath := range matchedEnvs {
			env := sm.g.environments[envPath]
			for appName, appProto := range env.foundApplications {
				if appProto == prototype {
					envAppMap[envPath] = append(envAppMap[envPath], appName)
				}
			}
		}
	}
	return envAppMap
}

// getEnvironmentsUnderRoot returns all environment paths that are under the given root
func (sm *SmartMode) getEnvironmentsUnderRoot(root string) []string {
	var matchedEnvs []string
	root = filepath.Clean(root)

	for envPath := range sm.g.environments {
		envPath = filepath.Clean(envPath)
		if envPath == root || strings.HasPrefix(envPath, root+string(filepath.Separator)) {
			matchedEnvs = append(matchedEnvs, envPath)
		}
	}

	return matchedEnvs
}

func getChanges(changedFilePaths []string, regExps ...string) ([]string, []string) {
	var matches1 []string
	var matches2 []string
	for _, expr := range regExps {
		for _, line := range changedFilePaths {
			expr := regexp.MustCompile(expr)
			matches := expr.FindStringSubmatch(line)
			if matches != nil {
				if len(matches) == 1 {
					matches1 = append(matches1, matches[0])
				} else if len(matches) == 2 {
					matches1 = append(matches1, matches[1])
				} else {
					matches1 = append(matches1, matches[1])
					matches2 = append(matches2, matches[2])
				}
			}
		}
	}
	return matches1, matches2
}
