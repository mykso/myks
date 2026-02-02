package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mykso/myks/internal/myks"
)

func getGlobe() *myks.Globe {
	if globe == nil {
		globe = myks.New(viper.GetString("root-dir"))
		if err := viper.UnmarshalKey("naming-conventions", globe); err != nil {
			log.Error().Err(err).Msg("Unable to unmarshal naming-conventions config")
		}
	}
	return globe
}

func okOrFatal(err error, msg string) {
	if err != nil {
		log.Fatal().Err(err).Msg(msg)
	}
}

func okOrErrLog(err error, msg string) error {
	if err != nil {
		log.Error().Err(err).Msg(msg)
	}
	return err
}

// shellCompletion provides shell completion for envs and apps selection.
// For the first argument, it returns:
// - Directory paths under envs/ (excluding _* prefixed directories)
// - Environment IDs from rendered/envs/
// For the second argument, it returns application names:
// - For environment IDs: from rendered/envs/<id>/ directory listing
// - For environment paths: from ytt data-values
func shellCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	g := getGlobe()

	// return envs (first argument)
	if len(args) == 0 {
		return getEnvCompletions(g), cobra.ShellCompDirectiveNoFileComp
	}

	// return apps (second argument)
	if len(args) == 1 {
		return getAppCompletions(g, args[0]), cobra.ShellCompDirectiveNoFileComp
	}

	return nil, cobra.ShellCompDirectiveNoFileComp
}

// getEnvCompletions returns environment completions combining:
// - Directory paths under envs/ (excluding _* prefixed directories)
// - Environment IDs from rendered/envs/
func getEnvCompletions(g *myks.Globe) []string {
	seen := make(map[string]struct{})
	var completions []string

	// Add directory paths from envs/
	for _, dir := range listEnvDirs(g) {
		if _, exists := seen[dir]; !exists {
			seen[dir] = struct{}{}
			completions = append(completions, dir)
		}
	}

	// Add environment IDs from rendered/envs/
	for _, envID := range listRenderedEnvIDs(g) {
		if _, exists := seen[envID]; !exists {
			seen[envID] = struct{}{}
			completions = append(completions, envID)
		}
	}

	return completions
}

// getAppCompletions returns application completions for comma-separated environment list.
// For environment IDs: uses fast directory listing from rendered/envs/<id>/
// For environment paths: uses ytt data-values (slower but necessary for unrendered envs)
func getAppCompletions(g *myks.Globe, envsArg string) []string {
	seen := make(map[string]struct{})
	var completions []string

	// Parse comma-separated environments
	for env := range strings.SplitSeq(envsArg, ",") {
		env = strings.TrimSpace(env)
		if env == "" || env == "ALL" {
			continue
		}

		var apps []string

		// Check if it's a rendered environment ID first (fast path)
		if g.IsRenderedEnvID(env) {
			apps = listRenderedAppsForEnvID(g, env)
		} else {
			// Fall back to ytt data-values for environment paths
			apps = getAppsFromYttDataValues(g, env)
		}

		// Add unique apps to completions
		for _, app := range apps {
			if _, exists := seen[app]; !exists {
				seen[app] = struct{}{}
				completions = append(completions, app)
			}
		}
	}

	return completions
}

// readFlagBool reads a boolean flag from a cobra command and returns the value and whether the flag was set
func readFlagBool(cmd *cobra.Command, name string) (bool, bool) {
	flag, err := cmd.Flags().GetBool(name)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read flag")
		// This should never happen
		return false, false
	}

	return flag, cmd.Flags().Changed(name)
}

// listEnvDirs returns all directory paths under envs/, excluding directories prefixed with underscore.
// This is a fast operation that only lists directories without any ytt processing.
func listEnvDirs(g *myks.Globe) []string {
	var envDirs []string
	baseDir := filepath.Join(g.RootDir, g.EnvironmentBaseDir)

	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d == nil || !d.IsDir() {
			return nil
		}

		// Skip directories prefixed with underscore
		if strings.HasPrefix(d.Name(), "_") {
			return fs.SkipDir
		}

		// Skip the base directory itself
		if path == baseDir {
			return nil
		}

		// Get relative path from base directory
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}
		envDirs = append(envDirs, relPath)
		return nil
	})
	if err != nil {
		log.Debug().Err(err).Msg("Unable to list environment directories")
		return nil
	}

	return envDirs
}

// listRenderedEnvIDs returns environment IDs from rendered/envs/ directory.
// This is a fast operation that only lists directories without any ytt processing.
func listRenderedEnvIDs(g *myks.Globe) []string {
	var envIDs []string
	renderedDir := filepath.Join(g.RootDir, g.RenderedEnvsDir)

	entries, err := os.ReadDir(renderedDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Debug().Err(err).Msg("Unable to read rendered environments directory")
		}
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			envIDs = append(envIDs, entry.Name())
		}
	}

	return envIDs
}

// listRenderedAppsForEnvID lists app directories under rendered/envs/<id>/.
// This is a fast operation that only lists directories without any ytt processing.
func listRenderedAppsForEnvID(g *myks.Globe, envID string) []string {
	var apps []string
	renderedEnvDir := filepath.Join(g.RootDir, g.RenderedEnvsDir, envID)

	entries, err := os.ReadDir(renderedEnvDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Debug().Err(err).Str("envID", envID).Msg("Unable to read rendered environment directory")
		}
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			apps = append(apps, entry.Name())
		}
	}

	return apps
}

// getAppsFromYttDataValues runs ytt to get applications for an environment path.
// This is slower than directory listing but necessary for unrendered environments.
func getAppsFromYttDataValues(g *myks.Globe, envPath string) []string {
	// Add base dir to env path if not already present
	fullEnvPath := g.AddBaseDirToEnvPath(envPath)

	apps, err := g.GetApplicationsForEnvPath(fullEnvPath)
	if err != nil {
		log.Debug().Err(err).Str("envPath", envPath).Msg("Unable to get applications from ytt data-values")
		return nil
	}

	return apps
}
