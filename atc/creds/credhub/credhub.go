package credhub

import (
	"path"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/bosh-cli/director/template"
)

type CredHubAtc struct {
	CredHub *LazyCredhub
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
			c.logger.Error("could not find cred", err)
			return nil, false, err
		}
	}

	if !found {
		cred, found, err = c.findCred(c.path(c.TeamName, varDef.Name))
		if err != nil {
			c.logger.Error("could not find cred", err)
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

	ch, err := c.CredHub.CredHub()
	if err != nil {
		return cred, false, err
	}

	_, err = ch.FindByPath(path)
	if err != nil {
		return cred, false, err
	}

	cred, err = ch.GetLatestVersion(path)
	if _, ok := err.(*credhub.Error); ok {
		return cred, false, nil
	}

	if err != nil {
		return cred, false, err
	}

	return cred, true, nil
}

func (c CredHubAtc) path(segments ...string) string {
	return path.Join(append([]string{c.PathPrefix}, segments...)...)
}

func (c CredHubAtc) List() ([]template.VariableDefinition, error) {
	// not implemented, see vault implementation
	return []template.VariableDefinition{}, nil
}

var _ template.Variables = new(CredHubAtc)
