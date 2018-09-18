package creds

import (
	"code.cloudfoundry.org/lager"
	flags "github.com/jessevdk/go-flags"
)

type Manager interface {
	IsConfigured() bool
	Validate() error
	Health() (*HealthResponse, error)
	Init(lager.Logger) error

	NewVariablesFactory(lager.Logger) (VariablesFactory, error)
}

type ManagerFactory interface {
	AddConfig(*flags.Group) Manager
}

type Managers map[string]Manager

type HealthResponse struct {
	Response interface{} `json:"response,omitempty"`
	Error    string      `json:"error,omitempty"`
	Method   string      `json:"method,omitempty"`
}

var managerFactories = map[string]ManagerFactory{}

func Register(name string, managerFactory ManagerFactory) {
	managerFactories[name] = managerFactory
}

func ManagerFactories() map[string]ManagerFactory {
	return managerFactories
}
