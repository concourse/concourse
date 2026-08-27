package dummy

import (
	"path"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"
)

var _ creds.Secrets = (*Secrets)(nil)

type Secrets struct {
	vars.StaticVariables

	TeamName     string
	PipelineName string
}

func (secrets *Secrets) NewSecretLookupPaths(params creds.SecretLookupParams, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}

	if len(params.Pipeline) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(path.Join(params.Team, params.Pipeline)+"/"))
	}

	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(params.Team+"/"))
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(""))

	return lookupPaths
}

func (secrets *Secrets) Get(secretPath string, _ creds.SecretLookupParams) (any, *time.Time, bool, error) {
	v, found, err := secrets.StaticVariables.Get(vars.Reference{Path: secretPath})
	if err != nil {
		return nil, nil, false, err
	}

	if found {
		return v, nil, true, nil
	}

	return nil, nil, false, nil
}
