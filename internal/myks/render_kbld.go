package myks

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v3"

	"github.com/mykso/myks/internal/locker"
)

const (
	kbldOverrideFileName = "kbld-overrides.yaml"
	kbldLockFilePrefix   = "kbld-lock-"
)

type Kbld struct {
	additive bool
	app      *Application
	ident    string
	locker   *locker.Locker
}

// NewKbldRenderer creates a Kbld renderer that resolves image references for the given application.
func NewKbldRenderer(app *Application, lock *locker.Locker) *Kbld {
	return &Kbld{
		additive: false,
		app:      app,
		ident:    "kbld",
		locker:   lock,
	}
}

// AcquireLock is a no-op for Kbld since it does not read from vendored sources.
func (k *Kbld) AcquireLock() (func(), error) {
	// No lock needed for kbld since it doesn't read any sources.
	return func() {}, nil
}

func (k *Kbld) IsAdditive() bool {
	return k.additive
}

func (k *Kbld) Ident() string {
	return k.ident
}

func (k *Kbld) Render(previousStepFile string) (string, error) {
	config, err := k.getKbldConfig()
	if err != nil {
		log.Warn().Err(err).Msg(k.app.Msg(k.getStepName(), "Unable to get kbld config"))
		return "", err
	}

	if !config.Enabled {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "Kbld is disabled in configuration, skipping"))
		// just read the previous step file and return its content
		// TODO: implement skipping for "rendering tools" properly
		data, err := os.ReadFile(filepath.Clean(previousStepFile))
		if err != nil {
			log.Warn().Err(err).Str("file", previousStepFile).Msg(k.app.Msg(k.getStepName(), "Unable to read previous step file"))
			return "", err
		}
		return string(data), nil
	}

	cmdArgs := []string{
		"kbld",
		"--file=" + previousStepFile,
		fmt.Sprintf("--images-annotation=%t", config.ImagesAnnotation),
	}

	// Check if there are any image overrides to apply and generate the overrides config file.
	// Image overrides should be added to the command after the lock file (if any), but the
	// overrides config is used to generate the lock file name, so we need to do this first.
	overridesFilePath := ""
	if len(config.Overrides) > 0 {
		overridesFilePath, err = k.generateOverridesConfig(previousStepFile, config)
		if err != nil {
			log.Warn().Err(err).Msg(k.app.Msg(k.getStepName(), "Unable to generate kbld overrides config"))
			return "", err
		}
	}

	// if cache is enabled, check existence of the lock file and include it in the args
	if config.Cache {
		// The lock file name is based on the hash of the overrides file (if any) and the
		// hash of any kbld source directories (if any). This forces kbld to re-resolve
		// images when either the overrides or the build context changes.
		sourcesDirPaths, err := extractKbldSourcePaths(previousStepFile)
		if err != nil {
			log.Warn().Err(err).Msg(k.app.Msg(k.getStepName(), "Unable to extract kbld source paths"))
			sourcesDirPaths = nil
		}
		lockFileName := k.getLockFileName(overridesFilePath, sourcesDirPaths)
		lockFilePath := k.app.expandServicePath(lockFileName)

		defer k.cleanupLockFiles(lockFileName)

		cmdArgs = append(cmdArgs, "--lock-output="+lockFilePath)

		if ok, err := isExist(lockFilePath); ok {
			log.Debug().Str("file", lockFilePath).Msg(k.app.Msg(k.getStepName(), "Using existing kbld lock file for caching"))
			cmdArgs = append(cmdArgs, "--file="+lockFilePath)
		} else if err == nil {
			log.Debug().Str("file", lockFilePath).Msg(k.app.Msg(k.getStepName(), "Kbld lock file not found, proceeding without cache"))
		} else {
			log.Warn().Err(err).Str("file", lockFilePath).Msg(k.app.Msg(k.getStepName(), "Error checking kbld lock file existence"))
		}
	}

	if overridesFilePath != "" {
		cmdArgs = append(cmdArgs, "--file="+overridesFilePath)
	}

	cmdLogFn := func(name string, err error, stderr string, args []string) {
		purpose := k.getStepName() + " render step"
		cmd := msgRunCmd(purpose, name, args)
		if err != nil {
			log.Error().Msg(cmd)
			log.Error().Msg(stderr)
		} else {
			log.Debug().Msg(cmd)
		}
	}
	res, err := runCmd(k.getStepName(), myksFullPath(), nil, cmdArgs, cmdLogFn)
	if err != nil {
		return "", err
	}

	if res.Stdout == "" {
		log.Warn().Msg(k.app.Msg(k.getStepName(), "Empty kbld output"))
		return "", nil
	}

	log.Info().Msg(k.app.Msg(k.getStepName(), "kbld rendered"))

	return res.Stdout, nil
}

func (k *Kbld) getKbldConfig() (KbldConfig, error) {
	dataValuesYaml, err := k.app.ytt(k.getStepName(), "get kbld config", k.app.yttDataFiles, "--data-values-inspect")
	if err != nil {
		return KbldConfig{}, err
	}

	return newKbldConfig(dataValuesYaml.Stdout)
}

// generateOverridesConfig detects images using kbld --unresolved-inspect,
// and generates a kbld config file with image overrides based on the provided KbldConfig.
func (k *Kbld) generateOverridesConfig(inputFile string, config KbldConfig) (string, error) {
	cmdArgs := []string{
		"kbld",
		"--file=" + inputFile,
		"--unresolved-inspect",
	}

	cmdLogFn := func(name string, err error, stderr string, args []string) {
		purpose := k.getStepName() + " detect images"
		cmd := msgRunCmd(purpose, name, args)
		if err != nil {
			log.Error().Msg(cmd)
			log.Error().Msg(stderr)
		} else {
			log.Debug().Msg(cmd)
		}
	}

	res, err := runCmd(k.getStepName(), myksFullPath(), nil, cmdArgs, cmdLogFn)
	if err != nil {
		return "", fmt.Errorf("failed to detect images: %w", err)
	}

	if res.Stdout == "" {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "No images detected by kbld"))
		return "", nil
	}

	prefix := "- image: "
	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	var imageRefs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, prefix); ok {
			imageRefs = append(imageRefs, after)
		}
	}

	if len(imageRefs) == 0 {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "No valid image references found"))
		return "", nil
	}

	overrideMap := make(map[string]string)
	for _, imageRef := range imageRefs {
		newImageRef, err := config.applyOverrides(imageRef)
		if err != nil {
			log.Warn().Err(err).Str("image", imageRef).Msg(k.app.Msg(k.getStepName(), "Failed to apply overrides"))
			continue
		}
		if newImageRef != "" {
			overrideMap[imageRef] = newImageRef
			log.Debug().Str("from", imageRef).Str("to", newImageRef).Msg(k.app.Msg(k.getStepName(), "Image override applied"))
		}
	}

	if len(overrideMap) == 0 {
		log.Debug().Msg(k.app.Msg(k.getStepName(), "No overrides applied"))
		return "", nil
	}

	return k.generateKbldOverrideConfig(overrideMap)
}

// generateKbldOverrideConfig creates a kbld config file with image overrides
func (k *Kbld) generateKbldOverrideConfig(overrides map[string]string) (string, error) {
	type kbldOverride struct {
		Image    string `yaml:"image"`
		NewImage string `yaml:"newImage"`
	}

	type kbldConfig struct {
		APIVersion string         `yaml:"apiVersion"`
		Kind       string         `yaml:"kind"`
		Overrides  []kbldOverride `yaml:"overrides"`
	}

	config := kbldConfig{
		APIVersion: "kbld.k14s.io/v1alpha1",
		Kind:       "Config",
	}

	// Sort overrides for stable order
	sortedKeys := make([]string, 0, len(overrides))
	for oldImage := range overrides {
		sortedKeys = append(sortedKeys, oldImage)
	}
	sort.Strings(sortedKeys)

	for _, oldImage := range sortedKeys {
		config.Overrides = append(config.Overrides, kbldOverride{
			Image:    oldImage,
			NewImage: overrides[oldImage],
		})
	}

	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal kbld config: %w", err)
	}

	err = k.app.writeServiceFile(kbldOverrideFileName, string(yamlBytes))
	filename := k.app.expandServicePath(kbldOverrideFileName)
	return filename, err
}

func (k *Kbld) getLockFileName(overridesFilePath string, sourcesDirPaths []string) string {
	// This is the FNV-1a 64-bit hash of an empty string ("").
	// It serves as the default hash value when no overrides file exists.
	overridesHash := "cbf29ce484222325"
	if overridesFilePath != "" {
		if data, err := hashFile(overridesFilePath); err == nil {
			overridesHash = data
		} else {
			log.Warn().Err(err).Msg(k.app.Msg(k.getStepName(), "Unable to hash overrides file for lock file naming"))
		}
	}

	// If no source directories, use existing single-hash format for backward compatibility.
	if len(sourcesDirPaths) == 0 {
		return fmt.Sprintf("%s%s.yaml", kbldLockFilePrefix, overridesHash)
	}

	// Combine per-directory hashes into a single sources hash.
	sourcesHasher := fnv.New64a()
	for _, dir := range sourcesDirPaths {
		dirHash, err := hashDirectory(dir)
		if err != nil {
			log.Warn().Err(err).Str("dir", dir).Msg(
				k.app.Msg(k.getStepName(), "Unable to hash source directory, skipping"))
			continue
		}
		if _, err := sourcesHasher.Write([]byte(dirHash)); err != nil {
			log.Warn().Err(err).Str("dir", dir).Msg(
				k.app.Msg(k.getStepName(), "Unable to update sources hash, skipping"))
		}
	}
	sourcesHash := fmt.Sprintf("%x", sourcesHasher.Sum64())

	return fmt.Sprintf("%s%s-%s.yaml", kbldLockFilePrefix, overridesHash, sourcesHash)
}

// extractKbldSourcePaths parses a multi-document YAML file (the previous render
// step output) and returns the deduplicated, sorted list of `sources[].path`
// values from kbld Config documents.
func extractKbldSourcePaths(previousStepFile string) ([]string, error) {
	file, err := os.Open(filepath.Clean(previousStepFile))
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("Failed to close file")
		}
	}()

	type kbldSourceConfig struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Sources    []struct {
			Path string `yaml:"path"`
		} `yaml:"sources"`
	}

	var paths []string
	decoder := yaml.NewDecoder(file)
	for {
		var doc kbldSourceConfig
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// Log a warning but continue — some documents may not conform to
			// our struct (e.g. Kubernetes resources with complex nested types).
			log.Warn().Err(err).Str("file", previousStepFile).Msg("Failed to decode YAML document while extracting kbld source paths")
			continue
		}
		if doc.APIVersion != "kbld.k14s.io/v1alpha1" || doc.Kind != "Config" {
			continue
		}
		for _, src := range doc.Sources {
			if src.Path != "" {
				paths = append(paths, src.Path)
			}
		}
	}

	sort.Strings(paths)
	paths = slices.Compact(paths)
	return paths, nil
}

func (k *Kbld) cleanupLockFiles(leaveFile string) {
	servicePath := k.app.expandServicePath("")
	entries, err := os.ReadDir(servicePath)
	if err != nil {
		log.Warn().Err(err).Str("path", servicePath).Msg(k.app.Msg(k.getStepName(), "Failed to read service directory for kbld lock file cleanup"))
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, kbldLockFilePrefix) {
			if name != leaveFile {
				fullPath := filepath.Join(servicePath, name)
				if err := os.Remove(fullPath); err != nil {
					log.Warn().Err(err).Str("file", fullPath).Msg(k.app.Msg(k.getStepName(), "Failed to remove kbld lock file during cleanup"))
				} else {
					log.Debug().Str("file", fullPath).Msg(k.app.Msg(k.getStepName(), "Removed kbld lock file during cleanup"))
				}
			}
		}
	}
}

func (k *Kbld) getStepName() string {
	return fmt.Sprintf("%s-%s", renderStepName, k.Ident())
}
