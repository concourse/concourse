package creds_test

import (
	"fmt"
	"time"

	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/credsfakes"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func makeFlakySecretManager(numberOfFails int) creds.Secrets {
	fakeSecretManager := new(credsfakes.FakeSecrets)
	attempt := 0
	fakeSecretManager.GetStub = func(vars.VariableReference) (interface{}, *time.Time, bool, error) {
		attempt++
		if attempt <= numberOfFails {
			return nil, nil, false, fmt.Errorf("remote error: handshake failure")
		}
		return "received value", nil, true, nil
	}
	return fakeSecretManager
}

var _ = Describe("Re-retrieval of secrets on retryable errors", func() {

	It("should retry receiving a parameter in case of retryable error", func() {
		flakySecretManager := makeFlakySecretManager(3)
		retryableSecretManager := creds.NewRetryableSecrets(flakySecretManager, creds.SecretRetryConfig{Attempts: 5, Interval: time.Millisecond})
		varDef := vars.VariableDefinition{Ref: vars.VariableReference{Name: "somevar"}}
		value, found, err := creds.NewVariables(retryableSecretManager, "team", "pipeline", false).Get(varDef)
		Expect(value).To(BeEquivalentTo("received value"))
		Expect(found).To(BeTrue())
		Expect(err).To(BeNil())
	})

	It("should not receive a parameter if the number of retryable errors exceeded the number of allowed attempts", func() {
		flakySecretManager := makeFlakySecretManager(10)
		retryableSecretManager := creds.NewRetryableSecrets(flakySecretManager, creds.SecretRetryConfig{Attempts: 5, Interval: time.Millisecond})
		varDef := vars.VariableDefinition{Ref: vars.VariableReference{Name: "somevar"}}
		value, found, err := creds.NewVariables(retryableSecretManager, "team", "pipeline", false).Get(varDef)
		Expect(value).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).NotTo(BeNil())
	})

})
