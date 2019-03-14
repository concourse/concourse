package config

import (
	"syscall"

	"github.com/concourse/concourse/atc"
)

func OverrideTaskParams(configFile []byte, args []string) (atc.TaskConfig, error) {
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
