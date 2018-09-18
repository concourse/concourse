package kubernetes

import (
	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/bosh-cli/director/template"
	v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Kubernetes struct {
	Clientset       *kubernetes.Clientset
	TeamName        string
	PipelineName    string
	NamespacePrefix string
	logger          lager.Logger
}

func (k Kubernetes) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	var namespace = k.NamespacePrefix + k.TeamName
	var pipelineSecretName = k.PipelineName + "." + varDef.Name
	var secretName = varDef.Name

	secret, found, err := k.findSecret(namespace, pipelineSecretName)

	if !found && err == nil {
		secret, found, err = k.findSecret(namespace, secretName)
	}

	if err != nil {
		k.logger.Error("k8s-secret-error", err, lager.Data{
			"namespace":          namespace,
			"pipelineSecretName": pipelineSecretName,
			"secretName":         secretName,
		})
		return nil, false, err
	}

	if found {
		return k.getValueFromSecret(secret)
	}

	k.logger.Info("k8s-secret-not-found", lager.Data{
		"namespace":          namespace,
		"pipelineSecretName": pipelineSecretName,
		"secretName":         secretName,
	})
	return nil, false, nil
}

func (k Kubernetes) getValueFromSecret(secret *v1.Secret) (interface{}, bool, error) {
	val, found := secret.Data["value"]
	if found {
		return string(val), true, nil
	}

	evenLessTyped := map[interface{}]interface{}{}
	for k, v := range secret.Data {
		evenLessTyped[k] = string(v)
	}

	return evenLessTyped, true, nil
}

func (k Kubernetes) findSecret(namespace, name string) (*v1.Secret, bool, error) {
	var secret *v1.Secret
	var err error

	secret, err = k.Clientset.Core().Secrets(namespace).Get(name, meta_v1.GetOptions{})

	if err != nil && k8s_errors.IsNotFound(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	} else {
		return secret, true, err
	}
}

func (k Kubernetes) List() ([]template.VariableDefinition, error) {
	// Unimplemented for Kubernetes secrets

	return []template.VariableDefinition{}, nil
}
