package rc

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"
)

type TargetProps struct {
	API      string `yaml:"api"`
	Username string
	Password string
	Cert     string
	Insecure bool
}

// TODO: Remove this function and put url logic in ATC Client
func (target TargetProps) URL() string {
	targetURL, _ := url.Parse(target.API)
	if target.Username != "" {
		targetURL.User = url.UserPassword(target.Username, target.Password)
	}
	return targetURL.String()
}

type targetDetailsYAML struct {
	Targets map[string]TargetProps
}

func NewTarget(api, username, password, cert string, insecure bool) TargetProps {
	return TargetProps{
		API:      strings.TrimRight(api, "/"),
		Username: username,
		Password: password,
		Cert:     cert,
		Insecure: insecure,
	}
}

func CreateOrUpdateTargets(targetName string, targetInfo TargetProps) error {
	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	flyTargets, err := loadTargets(flyrc)
	if err != nil {
		return err
	}

	if isURL(targetName) {
		return errors.New("The target name cannot begin with http:// or https://.")
	}

	flyTargets.Targets[targetName] = targetInfo

	return writeTargets(flyrc, flyTargets)
}

func SelectTarget(selectedTarget string, insecure bool) (*TargetProps, error) {
	if isURL(selectedTarget) {
		target := NewTarget(selectedTarget, "", "", "", insecure)
		return &target, nil
	} else {
		flyrc := filepath.Join(userHomeDir(), ".flyrc")
		flyTargets, err := loadTargets(flyrc)
		if err != nil {
			return nil, err
		}

		target, ok := flyTargets.Targets[selectedTarget]
		if !ok {
			return nil, fmt.Errorf("Unable to find target %s in %s", selectedTarget, flyrc)
		}

		target.Insecure = insecure
		return &target, nil
	}
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
		return &targetDetailsYAML{Targets: map[string]TargetProps{}}, nil
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

func isURL(passedURL string) bool {
	matched, _ := regexp.MatchString("^http[s]?://", passedURL)
	return matched
}
