package graph

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"gopkg.in/yaml.v2"
)

var taskRE = regexp.MustCompile("^[a-z0-9_-]+$")

type Trigger struct {
	Action    string `yaml:"action",json:"action"`
	Condition string `yaml:"condition",json:"condition"`

	// Email fields
	To        *string `yaml:"to",json:"to,omitempty"`
	Cc        *string `yaml:"cc",json:"cc,omitempty"`
	InReplyTo *string `yaml:"in_reply_to",json:"in_reply_to,omitempty"`

	// Webhook fields
	Url *string `yaml:"url",json:"url"`
}

type Manifest struct {
	Arch         *string                `yaml:"arch",json:"arch,omitempty"`
	Artifacts    []string               `yaml:"artifacts",json:"artifacts,omitempty"`
	Environment  map[string]interface{} `yaml:"environment",json:"environment,omitempty"`
	Image        string                 `yaml:"image",json:"image"`
	Packages     []string               `yaml:"packages",json:"packages,omitempty"`
	Repositories map[string]string      `yaml:"repositories",json:"repositories,omitempty"`
	Secrets      []string               `yaml:"secrets",json:"secrets,omitempty"`
	Shell        bool                   `yaml:"shell",json:"shell,omitempty"`
	Sources      []string               `yaml:"sources",json:"sources,omitempty"`
	Tasks        []map[string]string    `yaml:"tasks",json:"tasks"`
	Triggers     []Trigger              `yaml:"triggers",json:"triggers,omitempty"`
	OAuth        string                 `yaml:"oauth",json:"oauth,omitempty"`
}

func LoadManifest(in string) (*Manifest, error) {
	var manifest Manifest
	err := yaml.Unmarshal([]byte(in), &manifest)
	if err != nil {
		return nil, err
	}

	if manifest.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	for _, sec := range manifest.Secrets {
		_, err := uuid.Parse(sec)
		if err != nil {
			return nil, err
		}
	}

	artset := make(map[string]interface{})
	for _, art := range manifest.Artifacts {
		if _, ok := artset[art]; ok {
			return nil, fmt.Errorf("duplicate artifact %s", art)
		}
		artset[art] = nil
	}

	if len(manifest.Tasks) == 0 && !manifest.Shell {
		return nil, fmt.Errorf("list of tasks is required")
	}

	taskset := make(map[string]interface{})
	for _, task := range manifest.Tasks {
		if len(task) != 1 {
			return nil, fmt.Errorf(`task schema is {"name": "value"} (or 'taskname: script...' as YAML)`)
		}
		var name string
		for key, _ := range task {
			name = key
			break
		}

		if _, ok := taskset[name]; ok {
			return nil, fmt.Errorf("duplicate task %s", name)
		}
		taskset[name] = nil

		if !taskRE.Match([]byte(name)) {
			return nil, fmt.Errorf("invalid task name '%s'", name)
		}
	}

	for _, trigger := range manifest.Triggers {
		switch trigger.Action {
		case "email":
			if trigger.To == nil && trigger.Cc == nil {
				return nil, fmt.Errorf("email trigger requires 'to' or 'cc'")
			}
		case "webhook":
			if trigger.Url == nil {
				return nil, fmt.Errorf("webhook trigger requires 'url'")
			}
		default:
			return nil, fmt.Errorf("unknown trigger type '%s'", trigger.Action)
		}
	}

	return &manifest, nil
}
