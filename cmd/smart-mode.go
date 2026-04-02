package cmd

import (
	"encoding/json"
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

// AnnotationSmartMode is the commit annotation checked for enabling Smart Mode.
const (
	AnnotationSmartMode = "feat:smart-mode"
	AnnotationTrue      = "true"

	outputFormatText = "text"
	outputFormatJSON = "json"
)

// normalizeOnlyPrint normalizes the raw --smart-mode.only-print flag value.
// It handles legacy boolean aliases and validates the result.
// Returns "" (disabled), "text", or "json", or an error for unrecognized values.
func normalizeOnlyPrint(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "":
		return "", nil
	case "true", "1", "yes":
		return outputFormatText, nil
	case "false", "0", "no":
		return "", nil
	case outputFormatText, outputFormatJSON:
		return v, nil
	default:
		return "", fmt.Errorf("invalid value %q for --smart-mode.only-print; accepted values: text, json", v)
	}
}

func initTargetEnvsAndApps(_ *cobra.Command, args []string) (err error) {
	// Check positional arguments for Smart Mode:
	// 1. Comma-separated list of environment search paths/IDs or ALL to search everywhere (default: ALL)
	// 2. Comma-separated list of application names or none to process all applications (default: none)

	onlyPrint, err := normalizeOnlyPrint(viper.GetString("smart-mode.only-print"))
	if err != nil {
		return err
	}

	switch len(args) {
	case 0:
		g := getGlobe()
		envAppMap, err = g.DetectChangedEnvsAndApps(viper.GetString("smart-mode.base-revision"))
		if err != nil {
			if onlyPrint != "" {
				return fmt.Errorf("smart mode detection failed: %w", err)
			}
			log.Warn().Err(err).Msg("Unable to run Smart Mode. Rendering everything.")
		} else if len(envAppMap) == 0 {
			switch onlyPrint {
			case outputFormatJSON:
				fmt.Println("{}")
				os.Exit(0)
			case outputFormatText:
				fmt.Println(aurora.Bold("\nSmart Mode detected no changes."))
				os.Exit(0)
			default:
				log.Warn().Msg("Smart Mode did not find any changes. Exiting.")
				os.Exit(0)
			}
		}
	case 1:
		if args[0] != allEnvsToken {
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
		if args[1] != allEnvsToken {
			appNames = strings.Split(args[1], ",")
		}

		envAppMap = make(myks.EnvAppMap)
		if args[0] != allEnvsToken {
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

	if envAppMap == nil {
		g := getGlobe()
		envAppMap = myks.EnvAppMap{g.EnvironmentBaseDir: nil}
	}

	log.Debug().
		Interface("envAppMap", envAppMap).
		Msg("Parsed arguments")

	switch onlyPrint {
	case "":
		// not set, proceed to render
	case outputFormatJSON:
		out, err := json.MarshalIndent(envAppMap, "", "  ")
		if err != nil {
			return fmt.Errorf("unable to marshal env/app map to JSON: %w", err)
		}
		fmt.Println(string(out))
		os.Exit(0)
	case outputFormatText:
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
