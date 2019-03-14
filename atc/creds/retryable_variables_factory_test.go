package creds_test

import (
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc/creds"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

type flakyVariables struct {
	attempt       int
	numberOfFails int
}

func (f *flakyVariables) Get(template.VariableDefinition) (interface{}, bool, error) {
	f.attempt++
	if f.attempt <= f.numberOfFails {
		return nil, false, fmt.Errorf("remote error: handshake failure")
	}
	return "received value", true, nil
}

func (f *flakyVariables) List() ([]template.VariableDefinition, error) {
	return nil, nil
}

type flakyFactory struct {
	flakyVariables creds.Variables
}

func (f *flakyFactory) NewVariables(string, string) creds.Variables {
	return f.flakyVariables
}

var _ = Describe("Retryable Variables Factory", func() {

	It("should retry receiving a parameter in case of retryable error", func() {
		flakyFactory := &flakyFactory{flakyVariables: &flakyVariables{numberOfFails: 3}}
		factory := creds.NewRetryableVariablesFactory(flakyFactory, creds.SecretRetryConfig{Attempts: 5, Interval: time.Millisecond})
		varDef := template.VariableDefinition{Name: "somevar"}
		value, found, err := factory.NewVariables("team", "pipeline").Get(varDef)
		Expect(value).To(BeEquivalentTo("received value"))
		Expect(found).To(BeTrue())
		Expect(err).To(BeNil())
	})

	It("should not receive a parameter if the number of retryable errors exceeded the number of allowed attempts", func() {
		flakyFactory := &flakyFactory{flakyVariables: &flakyVariables{numberOfFails: 10}}
		factory := creds.NewRetryableVariablesFactory(flakyFactory, creds.SecretRetryConfig{Attempts: 5, Interval: time.Millisecond})
		varDef := template.VariableDefinition{Name: "somevar"}
		value, found, err := factory.NewVariables("team", "pipeline").Get(varDef)
		Expect(value).To(BeNil())
		Expect(found).To(BeFalse())
		Expect(err).NotTo(BeNil())
	})

})
