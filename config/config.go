package config

import (
	"fmt"
	"time"

	"github.com/concourse/turbine/api/builds"
)

type Config struct {
	Resources Resources `yaml:"resources"`
	Jobs      Jobs      `yaml:"jobs"`
}

type Resource struct {
	Name string `yaml:"name"`

	Type   string `yaml:"type"`
	Source Source `yaml:"source"`
}

type Source map[string]interface{}

type Job struct {
	Name string `yaml:"name"`

	BuildConfigPath string        `yaml:"build"`
	BuildConfig     builds.Config `yaml:"config"`

	Privileged bool `yaml:"privileged"`

	Serial bool `yaml:"serial"`

	Inputs  []Input  `yaml:"inputs"`
	Outputs []Output `yaml:"outputs"`
}

type Input struct {
	Resource    string   `yaml:"resource"`
	Passed      []string `yaml:"passed"`
	EachVersion bool     `yaml:"each_version"`
	DontCheck   bool     `yaml:"dont_check"`
}

type Output struct {
	Resource string `yaml:"resource"`
	Params   Params `yaml:"params"`
}

type Duration time.Duration

func (d *Duration) UnmarshalYAML(tag string, value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("invalid duration: %#v", value)
	}

	duration, err := time.ParseDuration(str)
	if err != nil {
		return err
	}

	*d = Duration(duration)

	return nil
}

type Params map[string]interface{}

type Resources []Resource

func (resources Resources) Lookup(name string) (Resource, bool) {
	for _, resource := range resources {
		if resource.Name == name {
			return resource, true
		}
	}

	return Resource{}, false
}

func (resources Resources) UpdateResource(resource Resource) Resources {
	newResources := make(Resources, len(resources))

	for i, oldResource := range resources {
		if oldResource.Name == resource.Name {
			newResources[i] = resource
		} else {
			newResources[i] = oldResource
		}
	}

	return newResources
}

type Jobs []Job

func (jobs Jobs) Lookup(name string) (Job, bool) {
	for _, job := range jobs {
		if job.Name == name {
			return job, true
		}
	}

	return Job{}, false
}
