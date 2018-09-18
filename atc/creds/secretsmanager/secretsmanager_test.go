package secretsmanager_test

import (
	"errors"
	"text/template"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	varTemplate "github.com/cloudfoundry/bosh-cli/director/template"

	. "github.com/concourse/atc/creds/secretsmanager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockSecretsManagerService struct {
	secretsmanageriface.SecretsManagerAPI

	stubGetParameter func(name string) (*secretsmanager.GetSecretValueOutput, error)
}

func (mock *MockSecretsManagerService) GetSecretValue(input *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
	if mock.stubGetParameter == nil {
		return nil, errors.New("stubGetParameter is not defined")
	}
	Expect(input).ToNot(BeNil())
	Expect(input.SecretId).ToNot(BeNil())
	value, err := mock.stubGetParameter(*input.SecretId)
	if err != nil {
		return nil, err
	}
	return value, nil
}

var _ = Describe("SecretsManager", func() {
	var secretAccess *SecretsManager
	var varDef varTemplate.VariableDefinition
	var mockService MockSecretsManagerService

	JustBeforeEach(func() {
		varDef = varTemplate.VariableDefinition{Name: "cheery"}
		t1, err := template.New("test").Parse(DefaultPipelineSecretTemplate)
		Expect(t1).NotTo(BeNil())
		Expect(err).To(BeNil())
		t2, err := template.New("test").Parse(DefaultTeamSecretTemplate)
		Expect(t2).NotTo(BeNil())
		Expect(err).To(BeNil())
		secretAccess = NewSecretsManager(lager.NewLogger("secretsmanager_test"), &mockService, "alpha", "bogus", []*template.Template{t1, t2})
		Expect(secretAccess).NotTo(BeNil())
		mockService.stubGetParameter = func(input string) (*secretsmanager.GetSecretValueOutput, error) {
			if input == "/concourse/alpha/bogus/cheery" {
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("secret value"), Name: &input}, nil
			}
			return nil, awserr.New(secretsmanager.ErrCodeResourceNotFoundException, "", nil)
		}
	})

	Describe("Get()", func() {
		It("should get parameter if exists", func() {
			value, found, err := secretAccess.Get(varDef)
			Expect(value).To(BeEquivalentTo("secret value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get complex parameter", func() {
			mockService.stubGetParameter = func(path string) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{
					SecretBinary: []byte(`{"name": "yours", "pass": "truely"}`),
				}, nil
			}
			value, found, err := secretAccess.Get(varTemplate.VariableDefinition{Name: "user"})
			Expect(err).To(BeNil())
			Expect(found).To(BeTrue())
			Expect(value).To(BeEquivalentTo(map[interface{}]interface{}{
				"name": "yours",
				"pass": "truely",
			}))
		})

		It("should get team parameter if exists", func() {
			mockService.stubGetParameter = func(input string) (*secretsmanager.GetSecretValueOutput, error) {
				if input != "/concourse/alpha/cheery" {
					return nil, awserr.New(secretsmanager.ErrCodeResourceNotFoundException, "", nil)
				}
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("team decrypted value")}, nil
			}
			value, found, err := secretAccess.Get(varDef)
			Expect(value).To(BeEquivalentTo("team decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return not found on error", func() {
			mockService.stubGetParameter = nil
			value, found, err := secretAccess.Get(varDef)
			Expect(value).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})

		It("should allow empty pipeline name", func() {
			secretAccess.PipelineName = ""
			mockService.stubGetParameter = func(input string) (*secretsmanager.GetSecretValueOutput, error) {
				Expect(input).To(Equal("/concourse/alpha/cheery"))
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("team power")}, nil
			}
			value, found, err := secretAccess.Get(varDef)
			Expect(value).To(BeEquivalentTo("team power"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})
})
