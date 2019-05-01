package kubernetes

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/creds"
	"k8s.io/client-go/kubernetes"
)

type kubernetesFactory struct {
	clientset       *kubernetes.Clientset
	logger          lager.Logger
	namespacePrefix string
}

func NewKubernetesFactory(logger lager.Logger, clientset *kubernetes.Clientset, namespacePrefix string) *kubernetesFactory {
	factory := &kubernetesFactory{
		clientset:       clientset,
		logger:          logger,
		namespacePrefix: namespacePrefix,
	}

	return factory
}

func (factory *kubernetesFactory) NewSecrets() creds.Secrets {
	return &Kubernetes{
		Clientset:       factory.clientset,
		logger:          factory.logger,
		namespacePrefix: factory.namespacePrefix,
	}
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (factory *kubernetesFactory) NewSecretLookupPaths(teamName string, pipelineName string) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	if len(pipelineName) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(factory.namespacePrefix+teamName+":"+pipelineName+"."))
	}
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(factory.namespacePrefix+teamName+":"))
	return lookupPaths
}
