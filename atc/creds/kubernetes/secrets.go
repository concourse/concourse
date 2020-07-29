package kubernetes

import (
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"

	v1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Secrets struct {
	logger lager.Logger

	client          kubernetes.Interface
	namespacePrefix string
}

// NewSecretLookupPaths defines how variables will be searched in the underlying secret manager
func (secrets Secrets) NewSecretLookupPaths(teamName string, pipelineName string, allowRootPath bool) []creds.SecretLookupPath {
	lookupPaths := []creds.SecretLookupPath{}
	if len(pipelineName) > 0 {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(secrets.namespacePrefix+teamName+"/"+pipelineName+"."))
	}
	lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(secrets.namespacePrefix+teamName+"/"))
	if allowRootPath {
		lookupPaths = append(lookupPaths, creds.NewSecretLookupWithPrefix(secrets.namespacePrefix+"/"))
	}
	return lookupPaths
}

// Get retrieves the value and expiration of an individual secret
func (secrets Secrets) Get(secretPath string) (interface{}, *time.Time, bool, error) {
	parts := strings.Split(secretPath, "/")
	if len(parts) != 2 {
		return nil, nil, false, fmt.Errorf("unable to split kubernetes secret path into [namespace]/[secret]: %s", secretPath)
	}

	var namespace = parts[0]
	var secretName = parts[1]

	secret, found, err := secrets.findSecret(namespace, secretName)
	if err != nil {
		secrets.logger.Error("failed-to-fetch-secret", err, lager.Data{
			"namespace":   namespace,
			"secret-name": secretName,
		})
		return nil, nil, false, err
	}

	if found {
		return secrets.getValueFromSecret(secret)
	}

	secrets.logger.Info("secret-not-found", lager.Data{
		"namespace":   namespace,
		"secret-name": secretName,
	})

	return nil, nil, false, nil
}

func (secrets Secrets) getValueFromSecret(secret *v1.Secret) (interface{}, *time.Time, bool, error) {
	val, found := secret.Data["value"]
	if found {
		return string(val), nil, true, nil
	}

	// TODO: make this smarter since we now have access to ref.Fields
	stringified := map[string]interface{}{}
	for k, v := range secret.Data {
		stringified[k] = string(v)
	}

	return stringified, nil, true, nil
}

func (secrets Secrets) findSecret(namespace, name string) (*v1.Secret, bool, error) {
	var secret *v1.Secret
	var err error

	secret, err = secrets.client.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})

	if err != nil && k8serr.IsNotFound(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	} else {
		return secret, true, err
	}
}
