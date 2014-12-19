package config

import (
	"io/ioutil"
	"log"
	"syscall"

	"github.com/concourse/atc"
	"gopkg.in/yaml.v2"
)

func LoadBuildConfig(configPath string, args []string) atc.BuildConfig {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalln("could not open config file:", err)
	}

	var config atc.BuildConfig

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
