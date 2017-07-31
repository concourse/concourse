package kubernetes

import (
	"fmt"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/bosh-cli/director/template"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Kubernetes struct {
	Clientset    *kubernetes.Clientset
	TeamName     string
	PipelineName string
	Logger       lager.Logger
}

func (k Kubernetes) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	// Look up secret ${TeamName}/${PipelineName}-concourse-secrets
	secret, err := k.findSecret(k.TeamName, k.defautSecretName())
	if err == nil {
		// Look up key ${varDef.Name} in secret
		if value, exists := secret.Data[varDef.Name]; exists {
			return string(value), true, nil
		}
		err = fmt.Errorf("key '%s' not found in secret", varDef.Name)
	}
	k.Logger.Debug("failed-to-load-k8s-secret", lager.Data{
		"namespace": k.TeamName,
		"name":      k.defautSecretName(),
		"error":     err.Error(),
	})

	// Look up secret ${TeamName}/${varDef.Name}
	secret, err = k.findSecret(k.TeamName, varDef.Name)
	if err == nil {
		// Can't look up nested path
		stringMap := map[string]string{}
		for k, v := range secret.Data {
			stringMap[k] = string(v)
		}
		return stringMap, true, nil
	}
	k.Logger.Debug("failed-to-load-k8s-secret", lager.Data{
		"namespace": k.TeamName,
		"name":      varDef.Name,
		"error":     err.Error(),
	})

	// give up. We never pass the error back as it's not clear which one to pass
	return nil, false, nil
}

func (k Kubernetes) defautSecretName() string {
	return k.PipelineName + "-concourse-secrets"
}

func (k Kubernetes) findSecret(namespace, name string) (*v1.Secret, error) {
	return k.Clientset.Core().Secrets(namespace).Get(name, meta_v1.GetOptions{})
}

func (k Kubernetes) List() ([]template.VariableDefinition, error) {
	// Don't think this works with vault.. if we need it to we'll figure it out
	// var defs []template.VariableDefinition

	// secret, err := v.vaultClient.List(v.PathPrefix)
	// if err != nil {
	// 	return defs, err
	// }

	// var def template.VariableDefinition
	// for name, _ := range secret.Data {
	// 	defs := append(defs, template.VariableDefinition{
	// 		Name: name,
	// 	})
	// }

	return []template.VariableDefinition{}, nil
}
