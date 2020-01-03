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
		fmt.Sprintf("Local credential file not found")
		return fmt.Errorf("Local credential file not found")
	}

	// Just go through the motions to make sure everything can be parsed/read/etc.
	yamlDoc, err := ioutil.ReadFile(manager.Path)
	if err != nil {
		// TODO: Proper error reporting
		fmt.Sprintf("Error reading YAML file: %s\n", err.Error())
		return fmt.Errorf("Error reading YAML file: %s\n", err.Error())
	}

	_, err = yaml.YAMLToJSON(yamlDoc)
	if err != nil {
		fmt.Sprintf("Error converting YAML to JSON: %s\n", err.Error())
		return fmt.Errorf("Error converting YAML to JSON: %s\n", err.Error())
	}

	// TODO: Add gjson.validate() (or similar)

	return nil
}

func (manager Manager) Health() (*creds.HealthResponse, error) {
	return &creds.HealthResponse{
		Method: "noop",
	}, nil
}

func (manager Manager) Close(logger lager.Logger) {
	// Anything here?
}

func (manager Manager) NewSecretsFactory(logger lager.Logger) (creds.SecretsFactory, error) {
	// TODO: Sanity check what/why this is happening here.
	if manager.SecretsFactory == nil {
		manager.SecretsFactory = &SecretsFactory{
			path:   manager.Path,
			logger: logger,
		}
	}

	return manager.SecretsFactory, nil
}
