package config

import (
	"io/ioutil"
	"log"
	"syscall"

	"github.com/concourse/turbine"
	"gopkg.in/yaml.v2"
)

func LoadConfig(configPath string, args []string) turbine.Config {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config turbine.Config

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
