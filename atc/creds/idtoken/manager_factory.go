package idtoken

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type managerFactory struct{}

var defaultAudience = []string{"concourse-pipeline-idp"}
var defaultTTL = 15 * time.Minute

func init() {
	creds.Register("idtoken", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &managerFactory{}
}

func (factory *managerFactory) AddConfig(group *flags.Group) creds.Manager {
	return &Manager{}
}

func (factory *managerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid idtoken provider config: %T", config)
	}

	return &Manager{
		Config: configMap,
	}, nil
}
