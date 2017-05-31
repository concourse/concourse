package config

import (
	"fmt"
	"io/ioutil"
	"syscall"

	"github.com/concourse/atc"
)

func LoadTaskConfig(configPath string, args []string) (atc.TaskConfig, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return atc.TaskConfig{}, fmt.Errorf("failed to read task config: %s", err)
	}

	config, err := atc.NewTaskConfig(configFile)
	if err != nil {
		return atc.TaskConfig{}, err
	}

	config.Run.Args = append(config.Run.Args, args...)

	for k := range config.Params {
		env, found := syscall.Getenv(k)
		if found {
			config.Params[k] = env
		}
	}

	return config, nil
}
