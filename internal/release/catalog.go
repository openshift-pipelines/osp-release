package release

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed components.yaml
var defaultCatalogYAML []byte

type componentSpec struct {
	Key         string `yaml:"key"`
	DisplayName string `yaml:"display_name"`
	Repo        string `yaml:"repo"`
}

type catalogSpec struct {
	DisplayNames map[string]string `yaml:"display_names"`
	Components   []componentSpec   `yaml:"components"`
}

type catalog struct {
	displayNames   map[string]string
	repoByName     map[string]string
	componentNames []string
}

var defaultCatalog = mustLoadCatalog(defaultCatalogYAML)

func mustLoadCatalog(raw []byte) catalog {
	var spec catalogSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		panic(fmt.Errorf("load component catalog: %w", err))
	}

	out := catalog{
		displayNames:   make(map[string]string, len(spec.DisplayNames)+len(spec.Components)),
		repoByName:     make(map[string]string, len(spec.Components)),
		componentNames: make([]string, 0, len(spec.Components)),
	}

	for key, value := range spec.DisplayNames {
		out.displayNames[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	for _, component := range spec.Components {
		key := strings.ToLower(strings.TrimSpace(component.Key))
		if key == "" {
			panic("load component catalog: component key is empty")
		}
		if component.Repo == "" {
			panic(fmt.Sprintf("load component catalog: component %q repo is empty", key))
		}
		if _, exists := out.repoByName[key]; exists {
			panic(fmt.Sprintf("load component catalog: duplicate component key %q", key))
		}
		out.repoByName[key] = strings.TrimSpace(component.Repo)
		out.displayNames[key] = strings.TrimSpace(component.DisplayName)
		out.componentNames = append(out.componentNames, key)
	}

	slices.Sort(out.componentNames)
	return out
}

func UpstreamRepo(component string) (string, bool) {
	repo, ok := defaultCatalog.repoByName[strings.ToLower(strings.TrimSpace(component))]
	return repo, ok
}

func UpstreamComponentNames() []string {
	return slices.Clone(defaultCatalog.componentNames)
}

func DisplayName(key string) string {
	if name, ok := defaultCatalog.displayNames[key]; ok {
		return name
	}
	return key
}
