package rc

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/concourse/atc"

	"gopkg.in/yaml.v2"
)

var ErrNoTargetSpecified = errors.New("no target specified")

type UnknownTargetError struct {
	TargetName TargetName
}

func (err UnknownTargetError) Error() string {
	return fmt.Sprintf("unknown target: %s", err.TargetName)
}

type TargetProps struct {
	API      string       `yaml:"api"`
	TeamName string       `yaml:"team"`
	Insecure bool         `yaml:"insecure,omitempty"`
	Token    *TargetToken `yaml:"token,omitempty"`
	CACert   string       `yaml:"ca_cert,omitempty"`
}

type TargetToken struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type targetDetailsYAML struct {
	Targets map[TargetName]TargetProps
}

func SaveTarget(
	targetName TargetName,
	api string,
	insecure bool,
	teamName string,
	token *TargetToken,
	caCert string,
) error {
	flyTargets, err := LoadTargets()
	if err != nil {
		return err
	}

	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	newInfo := flyTargets.Targets[targetName]
	newInfo.API = api
	newInfo.Insecure = insecure
	newInfo.Token = token
	newInfo.TeamName = teamName
	newInfo.CACert = caCert

	flyTargets.Targets[targetName] = newInfo
	return writeTargets(flyrc, flyTargets)
}

func selectTarget(selectedTarget TargetName) (TargetProps, error) {
	if selectedTarget == "" {
		return TargetProps{}, ErrNoTargetSpecified
	}
	flyTargets, err := LoadTargets()
	if err != nil {
		return TargetProps{}, err
	}

	target, ok := flyTargets.Targets[selectedTarget]
	if !ok {
		return TargetProps{}, UnknownTargetError{selectedTarget}
	}
	return target, nil
}

func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("USERPROFILE")
		if home == "" {
			home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		}

		if home == "" {
			panic("could not detect home directory for .flyrc")
		}

		return home
	}

	return os.Getenv("HOME")
}

func LoadTargets() (*targetDetailsYAML, error) {
	var flyTargets *targetDetailsYAML

	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	if _, err := os.Stat(flyrc); err == nil {
		flyTargetsBytes, err := ioutil.ReadFile(flyrc)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(flyTargetsBytes, &flyTargets)
		if err != nil {
			return nil, err
		}
	}

	if flyTargets == nil {
		return &targetDetailsYAML{Targets: map[TargetName]TargetProps{}}, nil
	}
	for name, targetProps := range flyTargets.Targets {
		if targetProps.TeamName == "" {
			targetProps.TeamName = atc.DefaultTeamName
			flyTargets.Targets[name] = targetProps
		}
	}
	return flyTargets, nil
}

func writeTargets(configFileLocation string, targetsToWrite *targetDetailsYAML) error {
	yamlBytes, err := yaml.Marshal(targetsToWrite)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configFileLocation, yamlBytes, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
