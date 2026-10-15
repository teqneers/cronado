// Package compose reads service labels from Docker Compose files.
package compose

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Service is a Compose service with the labels its container will get.
type Service struct {
	Name          string
	ContainerName string
	Labels        map[string]string
}

type file struct {
	Services map[string]struct {
		ContainerName string `yaml:"container_name"`
		Labels        labels `yaml:"labels"`
	} `yaml:"services"`
}

// labels accepts both Compose label syntaxes: a map, or a list of
// "key=value" strings.
type labels map[string]string

func (l *labels) UnmarshalYAML(node *yaml.Node) error {
	result := make(map[string]string)
	switch node.Kind {
	case yaml.MappingNode:
		var raw map[string]any
		if err := node.Decode(&raw); err != nil {
			return err
		}
		for k, v := range raw {
			if v == nil {
				result[k] = ""
				continue
			}
			result[k] = fmt.Sprint(v)
		}
	case yaml.SequenceNode:
		var raw []string
		if err := node.Decode(&raw); err != nil {
			return err
		}
		for _, entry := range raw {
			k, v, _ := strings.Cut(entry, "=")
			result[k] = v
		}
	default:
		return fmt.Errorf("line %d: labels must be a map or a list", node.Line)
	}
	*l = result
	return nil
}

// Load reads the services of one or more Compose files. Like
// "docker compose -f a.yml -f b.yml", later files add to and override
// the labels of earlier ones. Services are returned sorted by name.
func Load(paths ...string) ([]Service, error) {
	services := make(map[string]*Service)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var f file
		if err := yaml.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for name, svc := range f.Services {
			s, ok := services[name]
			if !ok {
				s = &Service{Name: name, Labels: make(map[string]string)}
				services[name] = s
			}
			if svc.ContainerName != "" {
				s.ContainerName = svc.ContainerName
			}
			for k, v := range svc.Labels {
				// Compose writes a literal $ as $$.
				s.Labels[k] = strings.ReplaceAll(v, "$$", "$")
			}
		}
	}

	result := make([]Service, 0, len(services))
	for _, s := range services {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
