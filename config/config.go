package config

import (
	"io/ioutil"
	"log"
	"syscall"

	"github.com/concourse/atc"
	"gopkg.in/yaml.v2"
)

func LoadTaskConfig(configPath string, args []string) atc.TaskConfig {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config atc.TaskConfig

	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalln("could not parse config file:", err)
	}

	config.Run.Args = append(config.Run.Args, args...)

	for k, _ := range config.Params {
		env, found := syscall.Getenv(k)
		if found {
			config.Params[k] = env
		}
	}

	return config
}
