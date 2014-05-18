package config

type Config struct {
	Resources map[string]Resource `yaml:"resources"`
	Jobs      map[string]Job      `yaml:"jobs"`
}

type Source map[string]interface{}

type Job struct {
	Privileged bool `yaml:"privileged"`

	BuildConfigPath string `yaml:"build"`

	Inputs map[string]Source `yaml:"inputs"`
}

type Resource struct {
	Type   string `yaml:"type"`
	Source Source `yaml:"source"`
}
