package rc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/concourse/go-concourse/concourse"
	"github.com/mattn/go-isatty"

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
	Insecure bool         `yaml:"insecure,omitempty"`
	Token    *TargetToken `yaml:"token,omitempty"`
}

type TargetToken struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

type targetDetailsYAML struct {
	Targets map[TargetName]TargetProps
}

func NewTarget(api string, insecure bool, token *TargetToken) TargetProps {
	return TargetProps{
		API:      strings.TrimRight(api, "/"),
		Insecure: insecure,
		Token:    token,
	}
}

func SaveTarget(targetName TargetName, api string, insecure bool, token *TargetToken) error {
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

func SelectTarget(selectedTarget TargetName) (TargetProps, error) {
	if selectedTarget == "" {
		return TargetProps{}, ErrNoTargetSpecified
	}

	flyrc := filepath.Join(userHomeDir(), ".flyrc")
	flyTargets, err := loadTargets(flyrc)
	if err != nil {
		return TargetProps{}, err
	}

	target, ok := flyTargets.Targets[selectedTarget]
	if !ok {
		return TargetProps{}, UnknownTargetError{selectedTarget}
	}

	return target, nil
}

func NewClient(atcURL string, insecure bool) concourse.Client {
	var tlsConfig *tls.Config
	if insecure {
		tlsConfig = &tls.Config{InsecureSkipVerify: insecure}
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		Proxy: http.ProxyFromEnvironment,
	}

	return concourse.NewClient(atcURL, &http.Client{
		Transport: transport,
	})
}

func TargetClient(selectedTarget TargetName) (concourse.Client, error) {
	targetClient, err := CommandTargetClient(selectedTarget, nil)
	if err != nil {
		return nil, err
	}
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Printf("targeting %s\n\n", targetClient.URL())
	}
	return targetClient, nil
}

func CommandTargetClient(selectedTarget TargetName, commandInsecure *bool) (concourse.Client, error) {
	target, err := SelectTarget(selectedTarget)
	if err != nil {
		return nil, err
	}

	var token *oauth2.Token
	if target.Token != nil {
		token = &oauth2.Token{
			TokenType:   target.Token.Type,
			AccessToken: target.Token.Value,
		}
	}

	var tlsConfig *tls.Config
	if commandInsecure != nil {
		tlsConfig = &tls.Config{InsecureSkipVerify: *commandInsecure}
	} else if target.Insecure {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	var transport http.RoundTripper

	transport = &http.Transport{
		TLSClientConfig: tlsConfig,
		Dial: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).Dial,
		Proxy: http.ProxyFromEnvironment,
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

	return concourse.NewClient(target.API, httpClient), nil
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
		return &targetDetailsYAML{Targets: map[TargetName]TargetProps{}}, nil
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
