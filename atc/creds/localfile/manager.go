package localfile

import (
	"code.cloudfoundry.org/lager"
	"encoding/json"
	"fmt"
	"github.com/concourse/concourse/atc/creds"
	"io/ioutil"
	"os"
	"sigs.k8s.io/yaml"
)

type Manager struct {
	Path string `long:"path" description:"Path to YAML credentials file."`

	SecretsFactory *SecretsFactory
}

func (manager *Manager) Init(logger lager.Logger) error {

	// Just go through the motions to make sure everything can be parsed/read/etc.
	yamlDoc, err := ioutil.ReadFile(manager.Path)
	if err != nil {
		return err
	}

	jsonDoc, err := yaml.YAMLToJSON(yamlDoc)
	if err != nil {
		fmt.Printf("Error converting YAML to JSON: %s\n", err.Error())
		return err
	}

	var someStruct map[string]interface{}
	err = json.Unmarshal(jsonDoc, &someStruct)
	if err != nil {
		fmt.Printf("Error unmarshaling JSON: %s\n", err.Error())
		return err
	}

	manager.SecretsFactory = &SecretsFactory{
		path:   manager.Path,
		logger: logger,
	}

	return nil
}

func (manager *Manager) MarshalJSON() ([]byte, error) {
	health, err := manager.Health()
	if err != nil {
		return nil, err
	}

	return json.Marshal(&map[string]interface{}{"health": health})
}

func (manager Manager) IsConfigured() bool {
	return len(manager.Path) > 0
}

func (manager Manager) Validate() error {
	_, err := os.Stat(manager.Path)
	if os.IsNotExist(err) {
		return fmt.Errorf("Local credential file not found")
	}

	return nil
}

func (manager Manager) Health() (*creds.HealthResponse, error) {
	return &creds.HealthResponse{
		Method: "noop",
	}, nil
}

func (manager Manager) Close(logger lager.Logger) {
}

func (manager Manager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
	if manager.SecretsFactory == nil {
		manager.SecretsFactory = &SecretsFactory{
			path:   manager.Path,
			logger: logger,
		}
	}

	return manager.SecretsFactory, nil
}
