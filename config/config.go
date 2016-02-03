package config

import (
	"fmt"
	"io/ioutil"
	"syscall"

	"github.com/concourse/atc"
	"gopkg.in/yaml.v2"
)

func LoadTaskConfig(configPath string, args []string) (atc.TaskConfig, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("failed to read task config: %s", err)
	}

	var config atc.TaskConfig

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("task config is malformed: %s", err)
	}

	config.Run.Args = append(config.Run.Args, args...)

	for k, _ := range config.Params {
		env, found := syscall.Getenv(k)
		if found {
			config.Params[k] = env
		}
	}

	return config, nil
}
