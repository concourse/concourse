package config

import "encoding/json"

type Config struct {
	Jobs Jobs `yaml:"jobs"`
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

type Job struct {
	Name string `yaml:"name"`

	Privileged bool `yaml:"privileged"`

	BuildConfigPath string `yaml:"build"`

	Inputs []Input `yaml:"inputs"`
}

func (job Job) UpdateInput(input Input) Job {
	newInputs := make([]Input, len(job.Inputs))

	for i, jinput := range job.Inputs {
		if jinput.Name == input.Name {
			newInputs[i] = input
		} else {
			newInputs[i] = jinput
		}
	}

	job.Inputs = newInputs

	return job
}

type Input struct {
	Name string `yaml:"name"`

	Type   string `yaml:"type"`
	Source Source `yaml:"source"`
}

type Source []byte

func (source *Source) UnmarshalYAML(tag string, data interface{}) error {
	sourceConfig := map[string]interface{}{}

	for k, v := range data.(map[interface{}]interface{}) {
		sourceConfig[k.(string)] = v
	}

	marshalled, err := json.Marshal(sourceConfig)
	if err != nil {
		return err
	}

	*source = marshalled

	return nil
}
