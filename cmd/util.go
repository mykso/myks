package cmd

import (
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/mykso/myks/internal/myks"
)

// shellCompletion provides shell completion for envs and apps selection
func shellCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	tmp := myks.New(".")
	err := tmp.Init(asyncLevel, map[string][]string{})
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	// return envs
	if len(args) == 0 {
		return getEnvNames(tmp), cobra.ShellCompDirectiveNoFileComp
	}
	// return args
	if len(args) == 1 {
		return getAppNamesForEnv(tmp, args[0]), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func getEnvNames(globe *myks.Globe) []string {
	var envNames []string
	for _, env := range globe.GetEnvs() {
		envNames = append(envNames, strings.TrimPrefix(env.Dir, globe.EnvironmentBaseDir+string(filepath.Separator)))
	}
	return envNames
}

func getAppNamesForEnv(globe *myks.Globe, envPath string) []string {
	env, ok := globe.GetEnvs()[globe.AddBaseDirToEnvPath(envPath)]
	if ok {
		return env.GetApplicationNames()
	}
	return []string{}
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
