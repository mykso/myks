package myks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/exp/maps"
)

func (g *Globe) DetectChangedEnvsAndApps(baseRevision string) (EnvAppMap, error) {
	// envAppMap is built later by calling g.runSmartMode
	_ = g.collectEnvironments(nil)

	err := process(0, g.environments, func(item interface{}) error {
		env, ok := item.(*Environment)
		if !ok {
			return fmt.Errorf("Unable to cast item to *Environment")
		}
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
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.EnvironmentDataFileName),
		},
		// Prototype name is the only submatch
		"prototype": {
			e(g.PrototypesDir + "/(.+)/" + pluginsPattern + "/.*"),
			e(g.PrototypesDir + "/(.+)/" + g.ApplicationDataFileName),
		},
		// Env path and app name are the submatches
		"app": {
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.AppsDir + "/([^/]+)/" + pluginsPattern + "/.*"),
			e("(" + g.EnvironmentBaseDir + ".*)/" + g.AppsDir + "/([^/]+)/" + g.ApplicationDataFileName),
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

	for _, path := range maps.Keys(changedFiles) {
		// Check if the global configuration has changed
		if extractMatches(exprMap["global"], path) != nil {
			// If global configuration has changed, we need to render all environments
			return EnvAppMap{g.EnvironmentBaseDir: nil}
		}

		// If env has changed
		if envMatch := extractMatches(exprMap["env"], path); envMatch != nil {
			changedEnvs = append(changedEnvs, envMatch[0])
			continue
		}

		// If prototype has changed
		if protoMatch := extractMatches(exprMap["prototype"], path); protoMatch != nil {
			changedPrototypes = append(changedPrototypes, protoMatch[0])
			continue
		}

		// If app has changed
		if appMatch := extractMatches(exprMap["app"], path); appMatch != nil {
			envAppMap[appMatch[0]] = append(envAppMap[appMatch[0]], appMatch[1])
			continue
		}
	}

	for env, apps := range g.findPrototypeUsage(changedPrototypes) {
		envAppMap[env] = append(envAppMap[env], apps...)
	}

	// If env has changed, all apps in that env are affected
	for _, env := range changedEnvs {
		envAppMap[env] = nil
	}

	for env, apps := range envAppMap {
		if apps != nil {
			// Remove duplicates
			envAppMap[env] = removeDuplicates(apps)
		}
	}

	// Remove environments and applications that are not found in the filesystem
	for env, apps := range envAppMap {
		// env can be an exact path of an environment or one of parent directories
		if !g.isEnvPath(env) {
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

func (g *Globe) findPrototypeUsage(prototypes []string) EnvAppMap {
	envAppMap := EnvAppMap{}
	for _, prototype := range prototypes {
		for envPath, env := range g.environments {
			for appName, appProto := range env.foundApplications {
				if appProto == prototype {
					envAppMap[envPath] = append(envAppMap[envPath], appName)
				}
			}
		}
	}
	return envAppMap
}

func checkFileChanged(changedFiles []string, regExps ...string) bool {
	for _, expr := range regExps {
		changes, _ := getChanges(changedFiles, expr)
		if len(changes) > 0 {
			return true
		}
	}
	return false
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
