package ssm_test

import (
	"encoding/json"
	"errors"
	"strconv"

	"code.cloudfoundry.org/lager/lagertest"
	"gopkg.in/yaml.v2"

	"github.com/concourse/concourse/atc/creds"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	. "github.com/concourse/concourse/atc/creds/ssm"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

type mockPathResultPage struct {
	params map[string]string
	err    error
}

func (page mockPathResultPage) ToGetParametersByPathOutput() (*ssm.GetParametersByPathOutput, error) {
	if page.err != nil {
		return nil, page.err
	}
	params := make([]*ssm.Parameter, 0, len(page.params))
	for name, value := range page.params {
		params = append(params, &ssm.Parameter{
			Name:  aws.String(name),
			Value: aws.String(value),
		})
	}
	return &ssm.GetParametersByPathOutput{Parameters: params}, nil
}

type MockSsmService struct {
	ssmiface.SSMAPI

	stubGetParameter             func(name string) (string, error)
	stubGetParametersByPathPages func(path string) []mockPathResultPage
	stubListTagsForResource      func(id string) (map[string]string, error)
}

func (mock *MockSsmService) GetParameter(input *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	if mock.stubGetParameter == nil {
		return nil, errors.New("stubGetParameter is not defined")
	}
	Expect(input).ToNot(BeNil())
	Expect(input.Name).ToNot(BeNil())
	Expect(input.WithDecryption).To(PointTo(Equal(true)))
	value, err := mock.stubGetParameter(*input.Name)
	if err != nil {
		return nil, err
	}
	return &ssm.GetParameterOutput{Parameter: &ssm.Parameter{Value: &value}}, nil
}

func (mock *MockSsmService) GetParametersByPathPages(input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
	if mock.stubGetParametersByPathPages == nil {
		return errors.New("stubGetParametersByPathPages is not defined")
	}
	Expect(input).NotTo(BeNil())
	Expect(input.Path).NotTo(BeNil())
	Expect(input.Recursive).To(PointTo(Equal(true)))
	Expect(input.WithDecryption).To(PointTo(Equal(true)))
	Expect(input.MaxResults).To(PointTo(BeEquivalentTo(10)))
	allPages := mock.stubGetParametersByPathPages(*input.Path)
	for n, page := range allPages {
		params, err := page.ToGetParametersByPathOutput()
		if err != nil {
			return err
		}
		params.NextToken = aws.String(strconv.Itoa(n + 1))
		lastPage := (n == len(allPages)-1)
		if !fn(params, lastPage) {
			break
		}
	}
	return nil
}

func (mock *MockSsmService) ListTagsForResource(input *ssm.ListTagsForResourceInput) (*ssm.ListTagsForResourceOutput, error) {
	if mock.stubListTagsForResource == nil {
		return nil, errors.New("stubListTagsForResource is not defined")
	}
	Expect(input).ToNot(BeNil())
	Expect(input.ResourceId).ToNot(BeNil())
	Expect(input.ResourceType).To(Equal(aws.String(ssm.ResourceTypeForTaggingParameter)))

	var tags []*ssm.Tag
	tagMap, err := mock.stubListTagsForResource(*input.ResourceId)
	if err != nil {
		return nil, err
	}

	for k, v := range tagMap {
		tags = append(tags, &ssm.Tag{
			Key:   &k,
			Value: &v,
		})
	}

	return &ssm.ListTagsForResourceOutput{TagList: tags}, nil
}

var _ = Describe("Ssm", func() {
	var ssmAccess *Ssm
	var variables vars.Variables
	var varRef vars.Reference
	var mockService MockSsmService

	JustBeforeEach(func() {
		varRef = vars.Reference{Path: "cheery"}
		t1, err := creds.BuildSecretTemplate("t1", DefaultPipelineSecretTemplate)
		Expect(t1).NotTo(BeNil())
		Expect(err).To(BeNil())
		t2, err := creds.BuildSecretTemplate("t2", DefaultTeamSecretTemplate)
		Expect(t2).NotTo(BeNil())
		Expect(err).To(BeNil())
		ssmAccess = NewSsm(lagertest.NewTestLogger("ssm_test"), &mockService, []*creds.SecretTemplate{t1, t2})
		variables = creds.NewVariables(ssmAccess, "alpha", "bogus", false)
		Expect(ssmAccess).NotTo(BeNil())
		mockService.stubGetParameter = func(input string) (string, error) {
			if input == "/concourse/alpha/bogus/cheery" {
				return "ssm decrypted value", nil
			}
			return "", awserr.New(ssm.ErrCodeParameterNotFound, "", nil)
		}
		mockService.stubGetParametersByPathPages = func(path string) []mockPathResultPage {
			return []mockPathResultPage{}
		}
		mockService.stubListTagsForResource = func(id string) (map[string]string, error) {
			return map[string]string{}, nil
		}
	})

	Describe("Get()", func() {
		It("should get parameter if exists", func() {
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("ssm decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get complex parameter", func() {
			mockService.stubGetParametersByPathPages = func(path string) []mockPathResultPage {
				return []mockPathResultPage{
					{
						params: map[string]string{
							"/concourse/alpha/bogus/user/name": "yours",
							"/concourse/alpha/bogus/user/pass": "truly",
						},
						err: nil,
					},
				}
			}
			value, found, err := variables.Get(vars.Reference{Path: "user"})
			Expect(value).To(BeEquivalentTo(map[string]interface{}{
				"name": "yours",
				"pass": "truly",
			}))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return numbers as strings", func() {
			mockService.stubGetParameter = func(input string) (string, error) {
				return "101", nil
			}
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("101"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get team parameter if exists", func() {
			mockService.stubGetParameter = func(input string) (string, error) {
				if input != "/concourse/alpha/cheery" {
					return "", awserr.New(ssm.ErrCodeParameterNotFound, "", nil)
				}
				return "team decrypted value", nil
			}
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("team decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return not found on error", func() {
			mockService.stubGetParameter = nil
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeNil())
			Expect(found).To(BeFalse())
			Expect(err).NotTo(BeNil())
		})

		It("should allow empty pipeline name", func() {
			variables := creds.NewVariables(ssmAccess, "alpha", "", false)
			mockService.stubGetParameter = func(input string) (string, error) {
				Expect(input).To(Equal("/concourse/alpha/cheery"))
				return "team power", nil
			}
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("team power"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should still read even when unable to list tags", func() {
			mockService.stubListTagsForResource = nil
			value, found, err := variables.Get(varRef)
			Expect(value).To(BeEquivalentTo("ssm decrypted value"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should return a json object if the json format tag is set", func() {
			object := map[string]interface{}{
				"hello": "world",
				"nested": map[string]interface{}{
					"array": []interface{}{1.2, 3.4, 5.6, 7.8},
				},
				"testing": true,
			}
			objectData, err := json.Marshal(object)
			Expect(err).To(BeNil())
			objectText := string(objectData)

			mockService.stubGetParameter = func(input string) (string, error) {
				return objectText, nil
			}
			mockService.stubListTagsForResource = func(input string) (map[string]string, error) {
				return map[string]string{"format": "json"}, nil
			}

			value, found, err := variables.Get(varRef)
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(value).To(BeEquivalentTo(object))
		})

		It("should return a yaml object if the yaml format tag is set", func() {
			object := map[interface{}]interface{}{
				"hello": "world",
				"nested": map[interface{}]interface{}{
					"array": []interface{}{1.2, 3.4, 5.6, 7.8},
				},
				"testing": true,
			}
			objectData, err := yaml.Marshal(object)
			Expect(err).To(BeNil())
			objectText := string(objectData)

			mockService.stubGetParameter = func(input string) (string, error) {
				return objectText, nil
			}
			mockService.stubListTagsForResource = func(input string) (map[string]string, error) {
				return map[string]string{"format": "yaml"}, nil
			}

			value, found, err := variables.Get(varRef)
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(value).To(BeEquivalentTo(object))
		})

		It("should trim a value with extra spaces", func() {
			mockService.stubGetParameter = func(input string) (string, error) {
				return "  hello world  ", nil
			}
			mockService.stubListTagsForResource = func(input string) (map[string]string, error) {
				return map[string]string{"format": "trim"}, nil
			}

			value, found, err := variables.Get(varRef)
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(value).To(BeEquivalentTo("hello world"))
		})
	})
})
