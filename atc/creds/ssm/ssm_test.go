package ssm_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager/v3/lagertest"

	"github.com/concourse/concourse/atc/creds"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	. "github.com/concourse/concourse/atc/creds/ssm"
	"github.com/concourse/concourse/atc/creds/ssm/ssmfakes"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Ssm", func() {
	var ssmAccess *Ssm
	var variables vars.Variables
	var varRef vars.Reference
	var mockService *ssmfakes.FakeSsmAPI

	JustBeforeEach(func() {
		varRef = vars.Reference{Path: "cheery"}
		t1, err := creds.BuildSecretTemplate("t1", DefaultPipelineSecretTemplate)
		Expect(t1).NotTo(BeNil())
		Expect(err).To(BeNil())
		t2, err := creds.BuildSecretTemplate("t2", DefaultTeamSecretTemplate)
		Expect(t2).NotTo(BeNil())
		Expect(err).To(BeNil())

		mockService = &ssmfakes.FakeSsmAPI{}
		mockService.GetParameterStub = getParameterStub(func(name string) (*ssm.GetParameterOutput, error) {
			if name == "/concourse/alpha/bogus/cheery" {
				val := "ssm decrypted value"
				return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: &val}}, nil
			}
			return nil, &types.ParameterNotFound{}
		})

		mockService.GetParametersByPathStub = getParametersByPathStub(func(path string) (*ssm.GetParametersByPathOutput, error) {
			return &ssm.GetParametersByPathOutput{}, nil
		})

		ssmAccess = NewSsm(lagertest.NewTestLogger("ssm_test"), mockService, []*creds.SecretTemplate{t1, t2}, "/concourse/shared")

		variables = creds.NewVariables(ssmAccess, "alpha", "bogus", false)
		Expect(ssmAccess).NotTo(BeNil())
	})

	Describe("Get()", func() {
		It("should get parameter if exists", func() {
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("ssm decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get complex parameter", func() {
			mockService.GetParametersByPathStub = getParametersByPathStub(func(path string) (*ssm.GetParametersByPathOutput, error) {
				return &ssm.GetParametersByPathOutput{Parameters: []types.Parameter{
					{Name: aws.String("/concourse/alpha/bogus/user/name"), Value: aws.String("yours")},
					{Name: aws.String("/concourse/alpha/bogus/user/pass"), Value: aws.String("truely")},
				}}, nil
			})

			value, found, err := variables.Get(vars.Reference{Path: "user"})
			Expect(value).To(BeEquivalentTo(map[string]any{
				"name": "yours",
				"pass": "truely",
			}))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return numbers as strings", func() {
			mockService.GetParameterStub = getParameterStub(func(name string) (*ssm.GetParameterOutput, error) {
				return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: aws.String("101")}}, nil
			})

			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("101"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get team parameter if exists", func() {
			mockService.GetParameterStub = getParameterStub(func(name string) (*ssm.GetParameterOutput, error) {
				if name != "/concourse/alpha/bogus/cheery" {
					return nil, &types.ParameterNotFound{}
				}
				return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: aws.String("team decrypted value")}}, nil
			})

			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("team decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get shared parameter if exists", func() {
			mockService.GetParameterStub = getParameterStub(func(name string) (*ssm.GetParameterOutput, error) {
				if name != "/concourse/shared/cheery" {
					return nil, &types.ParameterNotFound{}
				}
				return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: aws.String("shared decrypted value")}}, nil
			})

			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("shared decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return not found on error", func() {
			mockService.GetParameterReturns(nil, fmt.Errorf("some error"))
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})

		It("should allow empty pipeline name", func() {
			variables := creds.NewVariables(ssmAccess, "alpha", "", false)
			mockService.GetParameterStub = getParameterStub(func(name string) (*ssm.GetParameterOutput, error) {
				Expect(name).To(Equal("/concourse/alpha/cheery"))
				return &ssm.GetParameterOutput{Parameter: &types.Parameter{Value: aws.String("team power")}}, nil
			})

			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("team power"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})
})

func getParameterStub(f func(string) (*ssm.GetParameterOutput, error)) func(context.Context, *ssm.GetParameterInput, ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	return func(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
		Expect(ctx).NotTo(BeNil())
		Expect(params).NotTo(BeNil())
		Expect(params.Name).NotTo(BeNil())
		Expect(params.WithDecryption).To(PointTo(Equal(true)))
		return f(*params.Name)
	}
}

func getParametersByPathStub(f func(string) (*ssm.GetParametersByPathOutput, error)) func(context.Context, *ssm.GetParametersByPathInput, ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	return func(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
		Expect(ctx).NotTo(BeNil())
		Expect(params).NotTo(BeNil())
		Expect(params.Path).NotTo(BeNil())
		Expect(params.Recursive).To(PointTo(Equal(true)))
		Expect(params.WithDecryption).To(PointTo(Equal(true)))
		Expect(params.MaxResults).To(PointTo(BeEquivalentTo(10)))
		return f(*params.Path)
	}
}
