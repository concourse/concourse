package rc

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v2"
)

type targetProps struct {
	API      string `yaml:"api"`
	Username string
	Password string
	Cert     string
}

type targetDetailsYAML struct {
	Targets map[string]targetProps
}

func NewTarget(api, username, password, cert string) targetProps {
	return targetProps{
		API:      api,
		Username: username,
		Password: password,
		Cert:     cert,
	}
}

func CreateOrUpdateTargets(targetName string, targetInfo targetProps) error {
	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	flyTargets, err := loadTargets(flyrc)
	if err != nil {
		return err
	}

	flyTargets.Targets[targetName] = targetInfo

	return writeTargets(flyrc, flyTargets)
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func loadTargets(configFileLocation string) (*targetDetailsYAML, error) {
	var flyTargets *targetDetailsYAML

	if _, err := os.Stat(configFileLocation); err == nil {
		flyTargetsBytes, err := ioutil.ReadFile(configFileLocation)
		if err != nil {
			return nil, fmt.Errorf("could not read %s", configFileLocation)
		}

		err = yaml.Unmarshal(flyTargetsBytes, &flyTargets)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal %s", configFileLocation)
		}
	}

	if flyTargets == nil {
		return &targetDetailsYAML{Targets: map[string]targetProps{}}, nil
	}

	return flyTargets, nil
}

func writeTargets(configFileLocation string, targetsToWrite *targetDetailsYAML) error {
	yamlBytes, err := yaml.Marshal(targetsToWrite)
	if err != nil {
		return fmt.Errorf("could not marshal %s", configFileLocation)
	}

	err = ioutil.WriteFile(configFileLocation, yamlBytes, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not write %s", configFileLocation)
	}

	return nil
}
