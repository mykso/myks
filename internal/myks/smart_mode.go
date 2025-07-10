package myks

import (
	"errors"
	"maps"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

func (g *Globe) DetectChangedEnvsAndApps(baseRevision string) (EnvAppMap, error) {
	if !g.WithGit {
		return nil, errors.New("git is unavailable")
	}

	// envAppMap is built later by calling g.runSmartMode
	_ = g.collectEnvironments(nil)

	err := process(0, maps.Values(g.environments), func(env *Environment) error {
		return env.initEnvData()
	})
	if err != nil {
		log.Err(err).Msg(g.Msg("Failed to collect environments"))
		return nil, err
	}

	changedFiles, err := GetChangedFilesGit(baseRevision)
	if err != nil {
		log.Err(err).Msg(g.Msg("Failed to get diff"))
		return nil, err
	}
	log.Trace().Interface("changedFiles", changedFiles).Msg(g.Msg("Detected changes"))
	envAppMap := g.runSmartMode(changedFiles)
	for env, apps := range envAppMap {
		log.Debug().Str("env", env).Strs("apps", apps).Msg(g.Msg("Detected changes"))
	}

	return envAppMap, nil
}

// find apps that are missing from rendered folder
func (g *Globe) missingApplications() (EnvAppMap, error) {
	envsWithMissingApps := EnvAppMap{}
	for path, e := range g.environments {
		missing, err := e.missingApplications()
		if err != nil {
			log.Err(err).Str("env", path).Msg(g.Msg("Failed to get missing applications"))
			return nil, err
		}
		if len(missing) > 0 {
			envsWithMissingApps[path] = missing
		}
	}

	return envsWithMissingApps, nil
}

func (g *Globe) runSmartMode(changedFiles ChangedFiles) EnvAppMap {
	e := func(sample string) *regexp.Regexp {
		return regexp.MustCompile("^" + g.GitPathPrefix + sample + "$")
	}

	globToRegexp := func(glob string) string {
		r := glob
		r = strings.ReplaceAll(r, ".", "\\.")
		r = strings.ReplaceAll(r, "*", ".*")
		return r
	}

	// Subdirectories of apps and prototypes are named after plugins
	plugins := []string{
		g.ArgoCDDataDirName,
		g.HelmStepDirName,
		g.StaticFilesDirName,
		g.VendirStepDirName,
		g.YttPkgStepDirName,
		g.YttStepDirName,
	}
	pluginsPattern := "(?:" + strings.Join(plugins, "|") + ")"

	exprMap := map[string][]*regexp.Regexp{
		// No submatches needed
		"global": {
			e(g.YttLibraryDirName + "/.*"),
		},
		// Env search path is the only submatch
		"env": {
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.EnvsDir + "/" + g.YttStepDirName + "/.*"),
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.EnvsDir + "/" + g.ArgoCDDataDirName + "/.*"),
			e("(" + g.EnvironmentBaseDir + ".*)/" + globToRegexp(g.EnvironmentDataFileName)),
		},
		// Prototype name is the only submatch
		"prototype": {
			e(g.PrototypesDir + "/(.+)/" + pluginsPattern + "/.*"),
			e(g.PrototypesDir + "/(.+)/" + globToRegexp(g.ApplicationDataFileName)),
		},
		// Env path and prototype name are the submatches
		"env-prototype": {
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.PrototypeOverrideDir + "/([^/]+)/" + pluginsPattern + "/.*"),
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.PrototypeOverrideDir + "/([^/]+)/" + globToRegexp(g.ApplicationDataFileName)),
		},
		// Env path and app name are the submatches
		"app": {
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.AppsDir + "/([^/]+)/" + pluginsPattern + "/.*"),
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.AppsDir + "/([^/]+)/" + globToRegexp(g.ApplicationDataFileName)),
		},
		// Env ID and app name are the submatches
		"rendered-app": {
			e(g.RenderedEnvsDir + "/([^/]+)/([^/]+)/.*"),
			e(g.RenderedArgoDir + "/([^/]+)/app-([^/]+)\\.yaml"),
		},
	}

	extractMatches := func(exprs []*regexp.Regexp, path string) []string {
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

	// Here we start collecting changed environments and applications,
	// starting with those that are missed from the rendered directory.
	envAppMap, err := g.missingApplications()
	if err != nil {
		log.Err(err).Msg(g.Msg("Failed to get missing applications"))
	}

	changedEnvs := []string{}
	changedPrototypes := []string{}

	for path := range changedFiles {
		// Check if the global configuration has changed
		if extractMatches(exprMap["global"], path) != nil {
			// If global configuration has changed, we need to render all environments
			return EnvAppMap{g.EnvironmentBaseDir: nil}
		}

		// If env has changed
		if envMatch := extractMatches(exprMap["env"], path); envMatch != nil {
			envPath := g.AddBaseDirToEnvPath(envMatch[0])
			changedEnvs = append(changedEnvs, envPath)
			continue
		}

		// If prototype has changed
		if protoMatch := extractMatches(exprMap["prototype"], path); protoMatch != nil {
			changedPrototypes = append(changedPrototypes, protoMatch[0])
			continue
		}

		// If environment-specific prototype has changed
		if envProtoMatch := extractMatches(exprMap["env-prototype"], path); envProtoMatch != nil {
			envPath := g.AddBaseDirToEnvPath(envProtoMatch[0])
			prototypeName := envProtoMatch[1]
			for env, apps := range g.findPrototypeUsage([]string{prototypeName}, envPath) {
				envAppMap[env] = append(envAppMap[env], apps...)
			}
			continue
		}

		// If app has changed
		if appMatch := extractMatches(exprMap["app"], path); appMatch != nil {
			envPath := g.AddBaseDirToEnvPath(appMatch[0])
			envAppMap[envPath] = append(envAppMap[envPath], appMatch[1])
			continue
		}

		// If rendered app has changed
		if appMatch := extractMatches(exprMap["rendered-app"], path); appMatch != nil {
			env, err := g.getEnvByID(appMatch[0])
			if err != nil {
				log.Err(err).Str("envID", appMatch[0]).Msg(g.Msg("Failed to get environment by ID"))
				continue
			}
			envPath := g.AddBaseDirToEnvPath(env.Dir)
			envAppMap[envPath] = append(envAppMap[envPath], appMatch[1])
			continue
		}
	}

	for env, apps := range g.findPrototypeUsage(changedPrototypes, "") {
		envAppMap[env] = append(envAppMap[env], apps...)
	}

	// If env has changed, all apps in that env are affected
	for _, env := range changedEnvs {
		envAppMap[env] = nil
	}

	for env, apps := range envAppMap {
		if apps != nil {
			// Remove duplicates
			envAppMap[env] = unique(apps)
		}
	}

	// Remove environments and applications that are not found in the filesystem
	for env, apps := range envAppMap {
		// env can be an exact path of an environment or one of parent directories
		matchedEnvs := g.getEnvironmentsUnderRoot(env)
		if len(matchedEnvs) == 0 {
			delete(envAppMap, env)
			continue
		}
		for _, app := range apps {
			// env can be absent in g.environments if it is a parent directory of an environment
			// in this case we can't easily check if app is present in env
			// TODO: implement smarter lookup logic instead
			if _, ok := g.environments[env]; !ok {
				continue
			}
			if _, ok := g.environments[env].foundApplications[app]; !ok {
				envAppMap[env] = filterSlice(envAppMap[env], func(s string) bool { return s != app })
			}
		}
	}

	return envAppMap
}

func (g *Globe) findPrototypeUsage(prototypes []string, envRoot string) EnvAppMap {
	envAppMap := EnvAppMap{}
	if envRoot == "" {
		envRoot = g.EnvironmentBaseDir
	}

	matchedEnvs := g.getEnvironmentsUnderRoot(envRoot)

	for _, prototype := range prototypes {
		for _, envPath := range matchedEnvs {
			env := g.environments[envPath]
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
func (g *Globe) getEnvironmentsUnderRoot(root string) []string {
	var matchedEnvs []string
	root = filepath.Clean(root)

	for envPath := range g.environments {
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
