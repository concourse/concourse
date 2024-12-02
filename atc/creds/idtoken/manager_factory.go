package idtoken

import (
	"fmt"

	"github.com/concourse/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type ManagerFactory struct {
	issuer string
}

func init() {
	creds.Register("idtoken", NewManagerFactory())
}

func NewManagerFactory() creds.ManagerFactory {
	return &ManagerFactory{}
}

func (factory *ManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	// can not be configured via atc settings
	return &Manager{}
}

func (factory *ManagerFactory) SetIssuer(issuer string) {
	factory.issuer = issuer
}

func (factory *ManagerFactory) NewInstance(config interface{}) (creds.Manager, error) {
	configMap, ok := config.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid idtoken provider config: %T", config)
	}
	if factory.issuer == "" {
		return nil, fmt.Errorf("issuer not set for idtoken provider")
	}
	return NewManager(factory.issuer, configMap)
}

// UpdateGlobalManagerFactory provides a way to modify the global idtoken.ManagerFactory and set additional settings
// Changes done here will apply to all idtoken.Manager instances that are created from then on
func UpdateGlobalManagerFactory(update func(*ManagerFactory)) {
	idTokenFactory := creds.ManagerFactories()["idtoken"]
	if idtokenFactory, is := idTokenFactory.(*ManagerFactory); is {
		update(idtokenFactory)
		creds.ManagerFactories()["idtoken"] = idTokenFactory
	}
}
