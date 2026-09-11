package kubernetes

import (
	"code.cloudfoundry.org/lager/v3"
	"k8s.io/client-go/kubernetes"

	"github.com/concourse/concourse/v8/atc/creds"
)

type kubernetesFactory struct {
	logger lager.Logger

	client                kubernetes.Interface
	namespacePrefix       string
	namespaceSharedSuffix string
}

func NewKubernetesFactory(logger lager.Logger, client kubernetes.Interface, namespacePrefix, namespaceSharedSuffix string) *kubernetesFactory {
	factory := &kubernetesFactory{
		logger:                logger,
		client:                client,
		namespacePrefix:       namespacePrefix,
		namespaceSharedSuffix: namespaceSharedSuffix,
	}

	return factory
}

func (factory *kubernetesFactory) NewSecrets() creds.Secrets {
	return &Secrets{
		logger:                factory.logger,
		client:                factory.client,
		namespacePrefix:       factory.namespacePrefix,
		namespaceSharedSuffix: factory.namespaceSharedSuffix,
	}
}
