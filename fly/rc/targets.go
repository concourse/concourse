package rc

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/concourse/concourse/atc"

	"sigs.k8s.io/yaml"
)

var (
	ErrNoTargetSpecified = errors.New("no target specified")
	ErrNoTargetFromURL   = errors.New("no target matching url")
)

type UnknownTargetError struct {
	TargetName TargetName
}

func (err UnknownTargetError) Error() string {
	return fmt.Sprintf("unknown target: %s", err.TargetName)
}

type Targets map[TargetName]TargetProps

type RC struct {
	Targets Targets `json:"targets"`
}

type TargetProps struct {
	API            string       `json:"api"`
	TeamName       string       `json:"team"`
	Insecure       bool         `json:"insecure,omitempty"`
	Token          *TargetToken `json:"token,omitempty"`
	CACert         string       `json:"ca_cert,omitempty"`
	ClientCertPath string       `json:"client_cert_path,omitempty"`
	ClientKeyPath  string       `json:"client_key_path,omitempty"`
}

type TargetToken struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func flyrcPath() string {
	return filepath.Join(userHomeDir(), ".flyrc")
}

func LogoutTarget(targetName TargetName) error {
	flyTargets, err := LoadTargets()
	if err != nil {
		return err
	}

	if target, ok := flyTargets[targetName]; ok {
		if target.Token != nil {
			*target.Token = TargetToken{}
		}
	}

	return writeTargets(flyrcPath(), flyTargets)
}

func DeleteTarget(targetName TargetName) error {
	flyTargets, err := LoadTargets()
	if err != nil {
		return err
	}

	delete(flyTargets, targetName)

	return writeTargets(flyrcPath(), flyTargets)
}

func DeleteAllTargets() error {
	return writeTargets(flyrcPath(), Targets{})
}

func UpdateTargetProps(targetName TargetName, targetProps TargetProps) error {
	flyTargets, err := LoadTargets()
	if err != nil {
		return err
	}

	target := flyTargets[targetName]

	if targetProps.API != "" {
		target.API = targetProps.API
	}

	if targetProps.TeamName != "" {
		target.TeamName = targetProps.TeamName
	}

	flyTargets[targetName] = target

	return writeTargets(flyrcPath(), flyTargets)
}

func UpdateTargetName(targetName TargetName, newTargetName TargetName) error {
	flyTargets, err := LoadTargets()
	if err != nil {
		return err
	}

	if newTargetName != "" {
		flyTargets[newTargetName] = flyTargets[targetName]
		delete(flyTargets, targetName)
	}

	return writeTargets(flyrcPath(), flyTargets)
}

func SaveTarget(
	targetName TargetName,
	api string,
	insecure bool,
	teamName string,
	token *TargetToken,
	caCert string,
	clientCertPath string,
	clientKeyPath string,
) error {
	flyTargets, err := LoadTargets()
	if err != nil {
		return err
	}

	flyrc := flyrcPath()
	newInfo := flyTargets[targetName]
	newInfo.API = api
	newInfo.Insecure = insecure
	newInfo.Token = token
	newInfo.TeamName = teamName
	newInfo.CACert = caCert
	newInfo.ClientCertPath = clientCertPath
	newInfo.ClientKeyPath = clientKeyPath

	flyTargets[targetName] = newInfo
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

	target, ok := flyTargets[selectedTarget]
	if !ok {
		return TargetProps{}, UnknownTargetError{selectedTarget}
	}
	return target, nil
}

func userHomeDir() string {
	home := os.Getenv("FLY_HOME")
	if home != "" {
		return home
	}

	home = os.Getenv("HOME")
	if home != "" {
		return home
	}

	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
		if home != "" {
			return home
		}

		home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home != "" {
			return home
		}
	}

	panic("could not detect home directory for .flyrc")
}

func LoadTargets() (Targets, error) {
	var rc RC

	flyrc := flyrcPath()
	if _, err := os.Stat(flyrc); err == nil {
		flyTargetsBytes, err := ioutil.ReadFile(flyrc)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(flyTargetsBytes, &rc)
		if err != nil {
			return nil, fmt.Errorf("in the file '%s': %s", flyrc, err)
		}
	}

	targets := rc.Targets
	if targets == nil {
		targets = map[TargetName]TargetProps{}
	}

	for name, targetProps := range targets {
		if targetProps.TeamName == "" {
			targetProps.TeamName = atc.DefaultTeamName
			targets[name] = targetProps
		}
	}

	return targets, nil
}

func writeTargets(configFileLocation string, targetsToWrite Targets) error {
	yamlBytes, err := yaml.Marshal(RC{Targets: targetsToWrite})
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configFileLocation, yamlBytes, os.FileMode(0600))
	if err != nil {
		return err
	}

	return nil
}
