package kubernetes

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/creds"
	"k8s.io/client-go/kubernetes"
)

type kubernetesFactory struct {
	clientset *kubernetes.Clientset
	logger    lager.Logger
}

func NewKubernetesFactory(logger lager.Logger, clientset *kubernetes.Clientset) *kubernetesFactory {
	factory := &kubernetesFactory{
		clientset: clientset,
		logger:    logger,
	}

	return factory
}

func (factory *kubernetesFactory) NewVariables(teamName string, pipelineName string) creds.Variables {
	return &Kubernetes{
		Clientset:    factory.clientset,
		TeamName:     teamName,
		PipelineName: pipelineName,
		logger:       factory.logger,
	}
}
