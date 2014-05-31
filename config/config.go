package config

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

	Privileged bool `yaml:"privileged"`

	BuildConfigPath string `yaml:"build"`

	Inputs  []Input  `yaml:"inputs"`
	Outputs []Output `yaml:"outputs"`
}

type Input struct {
	Resource   string   `yaml:"resource"`
	Passed     []string `yaml:"passed"`
	LatestOnly bool     `yaml:"latest_only"`
}

type Output struct {
	Resource string `yaml:"resource"`
	Params   Params `yaml:"params"`
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
