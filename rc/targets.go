package rc

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/oauth2"

	"github.com/concourse/fly/atcclient"

	"gopkg.in/yaml.v2"
)

type TargetProps struct {
	API      string       `yaml:"api"`
	Insecure bool         `yaml:"insecure,omitempty"`
	Token    *TargetToken `yaml:"token,omitempty"`
}

type TargetToken struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type targetDetailsYAML struct {
	Targets map[string]TargetProps
}

func NewTarget(api string, insecure bool, token *TargetToken) TargetProps {
	return TargetProps{
		API:      strings.TrimRight(api, "/"),
		Insecure: insecure,
		Token:    token,
	}
}

func SaveTarget(targetName string, api string, insecure bool, token *TargetToken) error {
	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	flyTargets, err := loadTargets(flyrc)
	if err != nil {
		return err
	}

	newInfo := flyTargets.Targets[targetName]
	newInfo.API = api
	newInfo.Insecure = insecure
	newInfo.Token = token

	flyTargets.Targets[targetName] = newInfo

	return writeTargets(flyrc, flyTargets)
}

func SelectTarget(selectedTarget string) (TargetProps, error) {
	if isURL(selectedTarget) {
		return NewTarget(selectedTarget, false, nil), nil
	}

	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	flyTargets, err := loadTargets(flyrc)
	if err != nil {
		return TargetProps{}, err
	}

	target, ok := flyTargets.Targets[selectedTarget]
	if !ok {
		return TargetProps{}, fmt.Errorf("Unable to find target %s in %s", selectedTarget, flyrc)
	}

	return target, nil
}

func NewConnection(atcURL string, insecure bool) (atcclient.Connection, error) {
	var tlsConfig *tls.Config
	if insecure {
		tlsConfig = &tls.Config{InsecureSkipVerify: insecure}
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return atcclient.NewConnection(atcURL, &http.Client{
		Transport: transport,
	})
}

func TargetConnection(selectedTarget string) (atcclient.Connection, error) {
	if isURL(selectedTarget) {
		return NewConnection(selectedTarget, false)
	}

	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	flyTargets, err := loadTargets(flyrc)
	if err != nil {
		return nil, err
	}

	target, ok := flyTargets.Targets[selectedTarget]
	if !ok {
		return nil, fmt.Errorf("Unable to find target %s in %s", selectedTarget, flyrc)
	}

	var token *oauth2.Token
	if target.Token != nil {
		token = &oauth2.Token{
			TokenType:   target.Token.Type,
			AccessToken: target.Token.Value,
		}
	}

	var tlsConfig *tls.Config
	if target.Insecure {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	if token != nil {
		transport = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base:   transport,
		}
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	return atcclient.NewConnection(target.API, httpClient)
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
