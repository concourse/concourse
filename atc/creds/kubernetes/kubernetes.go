package kubernetes

import (
	"code.cloudfoundry.org/lager"
	"fmt"
	"github.com/concourse/concourse/atc/creds"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Kubernetes struct {
	Clientset       *kubernetes.Clientset
	logger          lager.Logger
	namespacePrefix string
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (k Kubernetes) NewSecretLookupPaths(teamName string, pipelineName string) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	if len(pipelineName) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(k.namespacePrefix+teamName+":"+pipelineName+"."))
	}
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(k.namespacePrefix+teamName+":"))
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (k Kubernetes) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	parts := strings.Split(secretPath, ":")
	if len(parts) != 2 {
		return nil, nil, false, fmt.Errorf("unable to split kubernetes secret path into [namespace]:[secret]: %s", secretPath)
	}

	var namespace = parts[0]
	var secretName = parts[1]

	secret, found, err := k.findSecret(namespace, secretName)

	if err != nil {
		k.logger.Error("unable to retrieve kubernetes secret", err, lager.Data{
			"namespace":  namespace,
			"secretName": secretName,
		})
		return nil, nil, false, err
	}

	if found {
		return k.getValueFromSecret(secret)
	}

	k.logger.Info("kubernetes secret not found", lager.Data{
		"namespace":  namespace,
		"secretName": secretName,
	})

	return nil, nil, false, nil
}

func (k Kubernetes) getValueFromSecret(secret *v1.Secret) (interface{}, *time.Time, bool, error) {
	val, found := secret.Data["value"]
	if found {
		return string(val), nil, true, nil
	}

	evenLessTyped := map[interface{}]interface{}{}
	for k, v := range secret.Data {
		evenLessTyped[k] = string(v)
	}

	return evenLessTyped, nil, true, nil
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
