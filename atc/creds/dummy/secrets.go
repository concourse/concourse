package dummy

import (
	"path"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"
)

type Secrets struct {
	vars.StaticVariables

	TeamName     string
	PipelineName string
}

func (secrets *Secrets) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}

	if len(pipelineName) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(teamName, pipelineName)+"/"))
	}

	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(teamName+"/"))
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(""))

	return lookupPaths
}

func (secrets *Secrets) Get(ref vars.VariableReference) (interface{}, *time.Time, bool, error) {
	v, found, err := secrets.StaticVariables.Get(vars.VariableDefinition{
		Ref: ref,
	})
	if err != nil {
		return nil, nil, false, err
	}

	if found {
		return v, nil, true, nil
	}

	return nil, nil, false, nil
}
