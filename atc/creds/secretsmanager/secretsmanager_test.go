package secretsmanager_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/secretsmanager/secretsmanagerfakes"
	"github.com/concourse/concourse/vars"

	. "github.com/concourse/concourse/atc/creds/secretsmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SecretsManager", func() {
	var secretAccess *SecretsManager
	var variables vars.Variables
	var varRef vars.Reference
	var mockService *secretsmanagerfakes.FakeSecretsManagerAPI

	JustBeforeEach(func() {
		varRef = vars.Reference{Path: "cheery"}
		t1, err := creds.BuildSecretTemplate("t1", DefaultPipelineSecretTemplate)
		Expect(t1).NotTo(BeNil())
		Expect(err).To(BeNil())
		t2, err := creds.BuildSecretTemplate("t2", DefaultTeamSecretTemplate)
		Expect(t2).NotTo(BeNil())
		Expect(err).To(BeNil())
		t3, err := creds.BuildSecretTemplate("t3", DefaultSharedSecretTemplate)
		Expect(t3).NotTo(BeNil())
		Expect(err).To(BeNil())

		mockService = &secretsmanagerfakes.FakeSecretsManagerAPI{}
		mockService.GetSecretValueStub = getSecretValueStub(func(secretID string) (*secretsmanager.GetSecretValueOutput, error) {
			if secretID == "/concourse/alpha/bogus/cheery" {
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("secret value"), Name: &secretID}, nil
			}
			return nil, &types.ResourceNotFoundException{}
		})

		secretAccess = NewSecretsManager(lagertest.NewTestLogger("secretsmanager_test"), mockService, []*creds.SecretTemplate{t1, t2, t3})
		variables = creds.NewVariables(secretAccess, creds.SecretLookupContext{Team: "alpha", Pipeline: "bogus"}, false)
		Expect(secretAccess).NotTo(BeNil())
	})

	Describe("Get()", func() {
		It("should get parameter if exists", func() {
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("secret value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get complex parameter", func() {
			mockService.GetSecretValueStub = getSecretValueStub(func(secretID string) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{SecretBinary: []byte(`{"name": "yours", "pass": "truely"}`), Name: &secretID}, nil
			})
			value, found, err := variables.Get(vars.Reference{Path: "user"})
			Expect(err).To(BeNil())
			Expect(found).To(BeTrue())
			Expect(value).To(BeEquivalentTo(map[string]any{
				"name": "yours",
				"pass": "truely",
			}))
		})

		It("should get json string parameter", func() {
			mockService.GetSecretValueStub = getSecretValueStub(func(secretID string) (*secretsmanager.GetSecretValueOutput, error) {
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"name": "yours", "pass": "truely"}`), Name: &secretID}, nil
			})
			value, found, err := variables.Get(vars.Reference{Path: "user"})
			Expect(err).To(BeNil())
			Expect(found).To(BeTrue())
			Expect(value).To(BeEquivalentTo(map[string]any{
				"name": "yours",
				"pass": "truely",
			}))
		})

		It("should get team parameter if exists", func() {
			mockService.GetSecretValueStub = getSecretValueStub(func(secretID string) (*secretsmanager.GetSecretValueOutput, error) {
				if secretID != "/concourse/alpha/cheery" {
					return nil, &types.ResourceNotFoundException{}
				}
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("team decrypted value"), Name: &secretID}, nil
			})
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("team decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return shared parameter if exists", func() {
			mockService.GetSecretValueStub = getSecretValueStub(func(secretID string) (*secretsmanager.GetSecretValueOutput, error) {
				if secretID != "/concourse/cheery" {
					return nil, &types.ResourceNotFoundException{}
				}
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("shared decrypted value"), Name: &secretID}, nil
			})
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("shared decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return not found on error", func() {
			mockService.GetSecretValueReturns(nil, fmt.Errorf("some error"))
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})

		It("should allow empty pipeline name", func() {
			variables := creds.NewVariables(secretAccess, creds.SecretLookupContext{Team: "alpha", Pipeline: ""}, false)
			mockService.GetSecretValueStub = getSecretValueStub(func(secretID string) (*secretsmanager.GetSecretValueOutput, error) {
				Expect(secretID).To(Equal("/concourse/alpha/cheery"))
				return &secretsmanager.GetSecretValueOutput{SecretString: aws.String("team power"), Name: &secretID}, nil
			})
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("team power"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should treat marked for deletion as deleted", func() {
			mockService.GetSecretValueStub = getSecretValueStub(func(secretID string) (*secretsmanager.GetSecretValueOutput, error) {
				return nil, &types.InvalidRequestException{}
			})
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).To(BeNil())
		})
	})
})

func getSecretValueStub(f func(secretID string) (*secretsmanager.GetSecretValueOutput, error)) func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	return func(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
		Expect(ctx).NotTo(BeNil())
		Expect(params).NotTo(BeNil())
		Expect(params.SecretId).NotTo(BeNil())
		return f(*params.SecretId)
	}
}
