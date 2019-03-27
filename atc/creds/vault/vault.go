package vault

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director/template"
	vaultapi "github.com/hashicorp/vault/api"
	"path"
	"time"
)

// A SecretReader reads a vault secret from the given path. It should
// be thread safe!
type SecretReader interface {
	Read(path string) (*vaultapi.Secret, error)
}

// Vault converts a vault secret to our completely untyped secret
// data.
type Vault struct {
	LoggedIn     <-chan struct{}
	SecretReader SecretReader

	PathPrefix   string
	TeamName     string
	PipelineName string
}

func (v Vault) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	var secret *vaultapi.Secret
	var found bool
	var err error

	if v.PipelineName != "" {
		secret, found, err = v.findSecret(v.path(v.TeamName, v.PipelineName, varDef.Name))
		if err != nil {
			return nil, false, err
		}
	}

	if !found {
		secret, found, err = v.findSecret(v.path(v.TeamName, varDef.Name))
		if err != nil {
			return nil, false, err
		}
	}

	if !found {
		return nil, false, nil
	}

	val, found := secret.Data["value"]
	if found {
		return val, true, nil
	}

	evenLessTyped := map[interface{}]interface{}{}
	for k, v := range secret.Data {
		evenLessTyped[k] = v
	}

	return evenLessTyped, true, nil
}

func (v Vault) getSecretReader() (SecretReader, error) {
	// this will wait for up to 5 seconds, until login into Vault gets established
	select {
	case <-v.LoggedIn:
		// if login into Vault is successful, v.LoggedIn channel will get closed
		// therefore this select won't block anymore since successful login
	case <-time.After(5 * time.Second):
		return nil, errors.New("timeout connecting to vault")
	}
	return v.SecretReader, nil
}

func (v Vault) findSecret(path string) (*vaultapi.Secret, bool, error) {
	sr, err := v.getSecretReader()
	if err != nil {
		return nil, false, fmt.Errorf("unable to retrieve '%s': %s", path, err)
	}

	secret, err := sr.Read(path)
	if err != nil {
		return nil, false, fmt.Errorf("unable to retrieve '%s': %s", path, err)
	}

	if secret != nil {
		return secret, true, nil
	}

	return nil, false, nil
}

func (v Vault) path(segments ...string) string {
	return path.Join(append([]string{v.PathPrefix}, segments...)...)
}

func (v Vault) List() ([]template.VariableDefinition, error) {
	// Don't think this works with vault.. if we need it to we'll figure it out
	// var defs []template.VariableDefinition

	// secret, err := v.vaultClient.List(v.PathPrefix)
	// if err != nil {
	// 	return defs, err
	// }

	// var def template.VariableDefinition
	// for name, _ := range secret.Data {
	// 	defs := append(defs, template.VariableDefinition{
	// 		Name: name,
	// 	})
	// }

	return []template.VariableDefinition{}, nil
}
