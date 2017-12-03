package ssm_test

import (
	"errors"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"
	. "github.com/concourse/atc/creds/ssm"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockSsmService struct {
	ssmiface.SSMAPI

	ExpectGetParameterInput  *ssm.GetParameterInput
	ReturnGetParameterOutput *ssm.GetParameterOutput
	ReturnGetParameterError  error
}

func (mock *MockSsmService) GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	Expect(input).To(BeEquivalentTo(mock.ExpectGetParameterInput))
	return mock.ReturnGetParameterOutput, mock.ReturnGetParameterError
}

var _ = Describe("Ssm", func() {
	var ssmAccess *Ssm
	var varDef varTemplate.VariableDefinition
	var mockService MockSsmService

	JustBeforeEach(func() {
		t, err := template.New("test").Parse("{{.Team}}-{{.Pipeline}}-{{.Secret}}")
		Expect(t).NotTo(BeNil())
		Expect(err).To(BeNil())
		ssmAccess = NewSsm(&mockService, "alpha", "bogus", t)
		Expect(ssmAccess).NotTo(BeNil())
		varDef = varTemplate.VariableDefinition{Name: "cheery"}
		mockService.ExpectGetParameterInput = &ssm.GetParameterInput{
			Name:           aws.String("alpha-bogus-cheery"),
			WithDecryption: aws.Bool(true),
		}
		mockService.ReturnGetParameterOutput = &ssm.GetParameterOutput{
			Parameter: &ssm.Parameter{Value: aws.String("ssm decrypted value")},
		}
	})

	Describe("Get()", func() {
		It("gets parameter successfully", func() {
			value, found, err := ssmAccess.Get(varDef)
			Expect(value).To(BeEquivalentTo("ssm decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return not found on error", func() {
			mockService.ReturnGetParameterError = errors.New("mock error")
			value, found, err := ssmAccess.Get(varDef)
			Expect(value).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})
	})
})
