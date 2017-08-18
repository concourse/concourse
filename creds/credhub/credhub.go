package credhub

import (
	"path"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/credentials"
	"github.com/cloudfoundry/bosh-cli/director/template"
)

type CredHubAtc struct {
	CredHub *credhub.CredHub
	logger  lager.Logger

	PathPrefix   string
	TeamName     string
	PipelineName string
}

func (c CredHubAtc) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	var cred credentials.Credential
	var found bool
	var err error

	if c.PipelineName != "" {
		path := c.path(c.TeamName, c.PipelineName, varDef.Name)
		cred, found, err = c.findCred(path)
		if err != nil {
			return nil, false, err
		}
	}

	if !found {
		cred, found, err = c.findCred(c.path(c.TeamName, varDef.Name))
		if err != nil {
			return nil, false, err
		}
	}

	if !found {
		return nil, false, nil
	}

	var result interface{} = cred.Value

	if standardMap, ok := cred.Value.(map[string]interface{}); ok {
		// TODO - we should do this recursively since the cpp4life go-path library
		// does not support map[string]interface{} types when looking for
		// nested values
		evenLessTyped := map[interface{}]interface{}{}

		for k, v := range standardMap {
			evenLessTyped[k] = v
		}

		result = evenLessTyped
	}

	return result, true, nil
}

func (c CredHubAtc) findCred(path string) (credentials.Credential, bool, error) {
	var cred credentials.Credential
	var err error

	c.logger.Info("credhub-get", lager.Data{"path ": path})

	_, err = c.CredHub.FindByPath(path)
	if err != nil {
		c.logger.Info("credhub-get", lager.Data{"error ": err})
		return cred, false, nil
	}

	cred, err = c.CredHub.Get(path)
	if err != nil {
		c.logger.Info("credhub-get", lager.Data{"error ": err})
		return cred, false, err
	}

	c.logger.Info("credhub-get", lager.Data{"cred ": cred.Value})

	return cred, true, nil
}

func (c CredHubAtc) path(segments ...string) string {
	return path.Join(append([]string{c.PathPrefix}, segments...)...)
}

func (c CredHubAtc) List() ([]template.VariableDefinition, error) {
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

var _ template.Variables = new(CredHubAtc)
