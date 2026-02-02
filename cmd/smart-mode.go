package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	aurora "github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/mykso/myks/internal/myks"
)

const (
	AnnotationSmartMode = "feat:smart-mode"
	AnnotationTrue      = "true"
)

func initTargetEnvsAndApps(_ *cobra.Command, args []string) (err error) {
	// Check positional arguments for Smart Mode:
	// 1. Comma-separated list of environment search paths/IDs or ALL to search everywhere (default: ALL)
	// 2. Comma-separated list of application names or none to process all applications (default: none)

	switch len(args) {
	case 0:
		g := getGlobe()
		envAppMap, err = g.DetectChangedEnvsAndApps(viper.GetString("smart-mode.base-revision"))
		if err != nil {
			log.Warn().Err(err).Msg("Unable to run Smart Mode. Rendering everything.")
		} else if len(envAppMap) == 0 {
			log.Warn().Msg("Smart Mode did not find any changes. Exiting.")
			os.Exit(0)
		}
	case 1:
		if args[0] != "ALL" {
			g := getGlobe()
			envAppMap = make(myks.EnvAppMap)
			for env := range strings.SplitSeq(args[0], ",") {
				// Resolve environment ID to path if needed
				resolvedEnv := g.ResolveEnvIdentifier(env)
				envAppMap[resolvedEnv] = nil
			}
		}
	case 2:
		var appNames []string
		if args[1] != "ALL" {
			appNames = strings.Split(args[1], ",")
		}

		envAppMap = make(myks.EnvAppMap)
		if args[0] != "ALL" {
			g := getGlobe()
			for env := range strings.SplitSeq(args[0], ",") {
				// Resolve environment ID to path if needed
				resolvedEnv := g.ResolveEnvIdentifier(env)
				envAppMap[resolvedEnv] = appNames
			}
		} else {
			g := getGlobe()
			envAppMap[g.EnvironmentBaseDir] = appNames
		}

	default:
		err := errors.New("too many positional arguments")
		log.Error().Err(err).Msg("Unable to parse positional arguments")
		return err
	}

	log.Debug().
		Interface("envAppMap", envAppMap).
		Msg("Parsed arguments")

	if viper.GetBool("smart-mode.only-print") {
		fmt.Println(aurora.Bold("\nSmart Mode detected:"))
		for env, apps := range envAppMap {
			fmt.Printf("→ %s\n", env)
			if apps == nil {
				fmt.Println(aurora.Bold("    ALL"))
			} else {
				for _, app := range apps {
					fmt.Printf("    %s\n", app)
				}
			}
		}
		os.Exit(0)
	}

	return nil
}
