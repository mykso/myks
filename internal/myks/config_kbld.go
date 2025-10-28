package myks

import (
	"fmt"
	"regexp"

	"github.com/google/go-containerregistry/pkg/name"
	yaml "gopkg.in/yaml.v3"
)

type KbldConfig struct {
	Enabled          bool               `yaml:"enabled"`
	ImagesAnnotation bool               `yaml:"imagesAnnotation"`
	Cache            bool               `yaml:"cache"`
	Overrides        []ImageRefOverride `yaml:"overrides"`
}

type ImageRefOverride struct {
	Match   ImageRefPattern `yaml:"match"`
	Replace ImageRefPattern `yaml:"replace"`

	registryRe   *regexp.Regexp
	repositoryRe *regexp.Regexp
	tagRe        *regexp.Regexp
}

type ImageRefPattern struct {
	Registry   string `yaml:"registry"`
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag"`
}

func newKbldConfig(dataValuesYaml string) (KbldConfig, error) {
	var kbldConfigWrapper struct {
		Kbld KbldConfig `yaml:"kbld"`
	}

	if err := yaml.Unmarshal([]byte(dataValuesYaml), &kbldConfigWrapper); err != nil {
		return KbldConfig{}, err
	}

	if err := kbldConfigWrapper.Kbld.initOverrides(); err != nil {
		return KbldConfig{}, err
	}

	return kbldConfigWrapper.Kbld, nil
}

func (cfg *KbldConfig) initOverrides() error {
	for i, override := range cfg.Overrides {
		if err := override.init(); err != nil {
			return fmt.Errorf("failed to initialize override %d: %w", i, err)
		}
		cfg.Overrides[i] = override
	}
	return nil
}

func (o *ImageRefOverride) init() error {
	if o.Match.Registry != "" {
		if registryRe, err := regexp.Compile("^" + o.Match.Registry + "$"); err == nil {
			o.registryRe = registryRe
		} else {
			return fmt.Errorf("invalid registry match regex %q: %w", o.Match.Registry, err)
		}
	}
	if o.Match.Repository != "" {
		if repositoryRe, err := regexp.Compile("^" + o.Match.Repository + "$"); err == nil {
			o.repositoryRe = repositoryRe
		} else {
			return fmt.Errorf("invalid repository match regex %q: %w", o.Match.Repository, err)
		}
	}
	if o.Match.Tag != "" {
		if tagRe, err := regexp.Compile("^" + o.Match.Tag + "$"); err == nil {
			o.tagRe = tagRe
		} else {
			return fmt.Errorf("invalid tag match regex %q: %w", o.Match.Tag, err)
		}
	}
	return nil
}

// applyOverrides applies the configured overrides to the given image reference
// and returns the modified image reference. Images from Docker Hub that are matched,
// are also always converted to their canonical form (i.e., adding "index.docker.io/library/").
// If no overrides match, an empty string is returned.
func (cfg *KbldConfig) applyOverrides(imageRef string) (string, error) {
	if len(cfg.Overrides) == 0 {
		return "", nil
	}

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference %q: %w", imageRef, err)
	}

	registry := ref.Context().RegistryStr()
	repository := ref.Context().RepositoryStr()
	tag := ""

	if tagged, ok := ref.(name.Tag); ok {
		tag = tagged.TagStr()
	} else if digested, ok := ref.(name.Digest); ok {
		// DigestStr() returns just the digest part without the @ prefix
		tag = "@" + digested.DigestStr()
	}

	for _, override := range cfg.Overrides {
		if !override.matches(registry, repository, tag) {
			continue
		}

		newRegistry, newRepository, newTag := override.apply(registry, repository, tag)
		if newRegistry == registry && newRepository == repository && newTag == tag {
			continue
		}

		newRef := newRegistry + "/" + newRepository

		if len(newTag) > 0 && newTag[0] == '@' {
			newRef += newTag
		} else {
			newRef += ":" + newTag
		}

		return newRef, nil
	}

	return "", nil
}

func (o *ImageRefOverride) matches(registry, repository, tag string) bool {
	return o.registryMatch(registry) &&
		o.repositoryMatch(repository) &&
		o.tagMatch(tag)
}

// apply applies the replacement pattern to the given image components
func (o *ImageRefOverride) apply(registry, repository, tag string) (string, string, string) {
	newRegistry := registry
	newRepository := repository
	newTag := tag

	if o.Replace.Registry != "" {
		newRegistry = o.Replace.Registry
		if o.registryRe != nil {
			newRegistry = o.registryRe.ReplaceAllString(registry, o.Replace.Registry)
		}
	}

	if o.Replace.Repository != "" {
		newRepository = o.Replace.Repository
		if o.repositoryRe != nil {
			newRepository = o.repositoryRe.ReplaceAllString(repository, o.Replace.Repository)
		}
	}

	if o.Replace.Tag != "" {
		newTag = o.Replace.Tag
		if o.tagRe != nil {
			newTag = o.tagRe.ReplaceAllString(tag, o.Replace.Tag)
		}
	}

	return newRegistry, newRepository, newTag
}

func (o *ImageRefOverride) registryMatch(registry string) bool {
	if o.registryRe == nil {
		return true
	}
	return o.registryRe.MatchString(registry)
}

func (o *ImageRefOverride) repositoryMatch(repository string) bool {
	if o.repositoryRe == nil {
		return true
	}
	return o.repositoryRe.MatchString(repository)
}

func (o *ImageRefOverride) tagMatch(tag string) bool {
	if o.tagRe == nil {
		return true
	}
	return o.tagRe.MatchString(tag)
}
