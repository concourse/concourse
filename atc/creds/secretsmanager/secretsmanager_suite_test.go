package secretsmanager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestSsm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SecretsManager Creds Suite")
}
