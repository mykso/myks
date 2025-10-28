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

	return kbldConfigWrapper.Kbld, nil
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
		// DigestStr() returns just the digest part without @
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
	reRegistry := o.reRegistry()
	reRepository := o.reRepository()
	reTag := o.reTag()

	return (reRegistry == nil || reRegistry.MatchString(registry)) &&
		(reRepository == nil || reRepository.MatchString(repository)) &&
		(reTag == nil || reTag.MatchString(tag))
}

// apply applies the replacement pattern to the given image components
func (o *ImageRefOverride) apply(registry, repository, tag string) (string, string, string) {
	newRegistry := registry
	newRepository := repository
	newTag := tag

	reRegistry := o.reRegistry()
	reRepository := o.reRepository()
	reTag := o.reTag()

	if o.Replace.Registry != "" {
		if reRegistry != nil {
			newRegistry = reRegistry.ReplaceAllString(registry, o.Replace.Registry)
		} else {
			newRegistry = o.Replace.Registry
		}
	}

	if o.Replace.Repository != "" {
		if reRepository != nil {
			newRepository = reRepository.ReplaceAllString(repository, o.Replace.Repository)
		} else {
			newRepository = o.Replace.Repository
		}
	}

	if o.Replace.Tag != "" {
		if reTag != nil {
			newTag = reTag.ReplaceAllString(tag, o.Replace.Tag)
		} else {
			newTag = o.Replace.Tag
		}
	}

	return newRegistry, newRepository, newTag
}

func (o *ImageRefOverride) reRegistry() *regexp.Regexp {
	if o.Match.Registry == "" {
		return nil
	}
	return regexp.MustCompile("^" + o.Match.Registry + "$")
}

func (o *ImageRefOverride) reRepository() *regexp.Regexp {
	if o.Match.Repository == "" {
		return nil
	}
	return regexp.MustCompile("^" + o.Match.Repository + "$")
}

func (o *ImageRefOverride) reTag() *regexp.Regexp {
	if o.Match.Tag == "" {
		return nil
	}
	return regexp.MustCompile("^" + o.Match.Tag + "$")
}
