package kubernetes

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/creds"
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

func (factory *kubernetesFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return &Kubernetes{
		Clientset:       factory.clientset,
		TeamName:        teamName,
		PipelineName:    pipelineName,
		NamespacePrefix: factory.namespacePrefix,
		logger:          factory.logger,
	}
}
