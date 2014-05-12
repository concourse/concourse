package config

type Config struct {
	Resources map[string]Resource `yaml:"resources"`
	Jobs      map[string]Job      `yaml:"jobs"`
}

type Job struct {
	Privileged bool `yaml:"privileged"`

	BuildConfigPath string `yaml:"build"`

	Inputs  map[string]ResourceVersion `yaml:"inputs"`
	Outputs map[string]ResourceUpdate  `yaml:"outputs"`
}

type Resource struct {
	Type string `yaml:"type"`
	URI  string `yaml:"uri"`
}

type ResourceVersion struct {
	Label  string `yaml:"label"`
	Branch string `yaml:"branch"`
}

type ResourceUpdate struct {
	Label string `yaml:"label"`
}
