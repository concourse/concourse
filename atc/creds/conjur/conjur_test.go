package conjur_test

import (
	"errors"

	"github.com/concourse/concourse/atc/creds"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/vars"

	. "github.com/concourse/concourse/atc/creds/conjur"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockConjurService struct {
	IConjurClient

	stubGetParameter func(name string) ([]byte, error)
}

func (mock *MockConjurService) RetrieveSecret(input string) ([]byte, error) {
	if mock.stubGetParameter == nil {
		return nil, errors.New("stubGetParameter is not defined")
	}
	Expect(input).ToNot(BeNil())
	value, err := mock.stubGetParameter(input)
	if err != nil {
		return nil, err
	}
	return value, nil
}

var _ = Describe("Conjur", func() {
	var secretAccess *Conjur
	var variables vars.Variables
	var varDef vars.VariableDefinition
	var mockService MockConjurService

	JustBeforeEach(func() {
		varDef = vars.VariableDefinition{Ref: vars.VariableReference{Name: "cheery"}}
		t1, err := creds.BuildSecretTemplate("t1", DefaultPipelineSecretTemplate)
		Expect(t1).NotTo(BeNil())
		Expect(err).To(BeNil())
		t2, err := creds.BuildSecretTemplate("t2", DefaultTeamSecretTemplate)
		Expect(t2).NotTo(BeNil())
		Expect(err).To(BeNil())
		secretAccess = NewConjur(lager.NewLogger("conjur_test"), &mockService, []*creds.SecretTemplate{t1, t2})
		variables = creds.NewVariables(secretAccess, "alpha", "bogus", false)
		Expect(secretAccess).NotTo(BeNil())
		mockService.stubGetParameter = func(input string) ([]byte, error) {
			if input == "concourse/alpha/bogus/cheery" {
				return []byte("secret value"), nil
			}
			return nil, errors.New("Variable not found")
		}
	})

	Describe("Get()", func() {
		It("should get parameter if exists", func() {
			value, found, err := variables.Get(varDef)
			Expect(value).To(BeEquivalentTo("secret value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get team parameter if exists", func() {
			mockService.stubGetParameter = func(input string) ([]byte, error) {
				if input != "concourse/alpha/cheery" {
					return nil, errors.New("Variable not found")
				}
				return []byte("team secret"), nil
			}
			value, found, err := variables.Get(varDef)
			Expect(value).To(BeEquivalentTo("team secret"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return not found on error", func() {
			mockService.stubGetParameter = nil
			value, found, err := variables.Get(varDef)
			Expect(value).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).To(BeNil())
		})

		It("should allow empty pipeline name", func() {
			variables := creds.NewVariables(secretAccess, "alpha", "", false)
			mockService.stubGetParameter = func(input string) ([]byte, error) {
				Expect(input).To(Equal("concourse/alpha/cheery"))
				return []byte("team secret"), nil
			}
			value, found, err := variables.Get(varDef)
			Expect(value).To(BeEquivalentTo("team secret"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should allow full variable path when no templates were configured", func() {
			secretAccess = NewConjur(lager.NewLogger("conjur_test"), &mockService, []*creds.SecretTemplate{})
			variables := creds.NewVariables(secretAccess, "", "", false)
			mockService.stubGetParameter = func(input string) ([]byte, error) {
				Expect(input).To(Equal("cheery"))
				return []byte("full path secret"), nil
			}
			value, found, err := variables.Get(varDef)
			Expect(value).To(BeEquivalentTo("full path secret"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})
})
