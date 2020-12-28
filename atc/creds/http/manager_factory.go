package http

import (
	"github.com/concourse/concourse/atc/creds"
	flags "github.com/jessevdk/go-flags"
)

type httpManagerFactory struct{}

func init() {
	creds.Register("http", NewHTTPManagerFactory())
}

func NewHTTPManagerFactory() creds.ManagerFactory {
	return &httpManagerFactory{}
}

func (factory *httpManagerFactory) AddConfig(group *flags.Group) creds.Manager {
	manager := &HTTPManager{}

	subGroup, err := group.AddGroup("HTTP Credential Management", "", manager)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "http-credential-manager"

	return manager
}

func (factory *httpManagerFactory) NewInstance(interface{}) (creds.Manager, error) {
	return &HTTPManager{}, nil
}
