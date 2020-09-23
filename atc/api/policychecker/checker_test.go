package policychecker_test

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/policychecker"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PolicyChecker", func() {
	var (
		policyFilter policy.Filter
		fakeAccess   *accessorfakes.FakeAccess
		fakeRequest  *http.Request
		result       policy.PolicyCheckOutput
		checkErr     error
	)

	BeforeEach(func() {
		fakeAccess = new(accessorfakes.FakeAccess)
		fakePolicyAgent = new(policyfakes.FakeAgent)
		fakePolicyAgentFactory.NewAgentReturns(fakePolicyAgent, nil)

		policyFilter = policy.Filter{
			ActionsToSkip: []string{},
			Actions:       []string{},
			HttpMethods:   []string{},
		}
	})

	JustBeforeEach(func() {
		policyCheck, err := policy.Initialize(testLogger, "some-cluster", "some-version", policyFilter)
		Expect(err).ToNot(HaveOccurred())
		Expect(policyCheck).ToNot(BeNil())
		result, checkErr = policychecker.NewApiPolicyChecker(policyCheck).Check("some-action", fakeAccess, fakeRequest)
	})

	Context("when system action", func() {
		BeforeEach(func() {
			fakeAccess.IsSystemReturns(true)
		})
		It("should pass", func() {
			Expect(checkErr).ToNot(HaveOccurred())
			Expect(result.Allowed).To(BeTrue())
		})
		It("Agent should not be called", func() {
			Expect(fakePolicyAgent.CheckCallCount()).To(Equal(0))
		})
	})

	Context("when not system action", func() {
		BeforeEach(func() {
			fakeAccess.IsSystemReturns(false)
		})

		Context("when the action should be skipped", func() {
			BeforeEach(func() {
				policyFilter.ActionsToSkip = []string{"some-action"}
			})
			It("should pass", func() {
				Expect(checkErr).ToNot(HaveOccurred())
				Expect(result.Allowed).To(BeTrue())
			})
			It("Agent should not be called", func() {
				Expect(fakePolicyAgent.CheckCallCount()).To(Equal(0))
			})
		})

		Context("when the http method no need to check", func() {
			BeforeEach(func() {
				fakeRequest = httptest.NewRequest("GET", "/something", nil)
				policyFilter.HttpMethods = []string{"PUT"}
			})
			It("should pass", func() {
				Expect(checkErr).ToNot(HaveOccurred())
				Expect(result.Allowed).To(BeTrue())
			})
			It("Agent should not be called", func() {
				Expect(fakePolicyAgent.CheckCallCount()).To(Equal(0))
			})
		})

		Context("when not in action list", func() {
			BeforeEach(func() {
				fakeRequest = httptest.NewRequest("PUT", "/something", nil)
				policyFilter.HttpMethods = []string{}
				policyFilter.Actions = []string{}
			})
			It("should pass", func() {
				Expect(checkErr).ToNot(HaveOccurred())
				Expect(result.Allowed).To(BeTrue())
			})
			It("Agent should not be called", func() {
				Expect(fakePolicyAgent.CheckCallCount()).To(Equal(0))
			})
		})

		Context("when the http method needs to check", func() {
			BeforeEach(func() {
				fakeRequest = httptest.NewRequest("PUT", "/something", nil)
				policyFilter.HttpMethods = []string{"PUT"}
			})

			Context("when request body is a bad json", func() {
				BeforeEach(func() {
					body := bytes.NewBuffer([]byte("hello"))
					fakeRequest = httptest.NewRequest("PUT", "/something", body)
					fakeRequest.Header.Add("Content-type", "application/json")
				})

				It("should error", func() {
					Expect(checkErr).To(HaveOccurred())
					Expect(checkErr.Error()).To(Equal(`invalid character 'h' looking for beginning of value`))
					Expect(result.Allowed).To(BeFalse())
				})
				It("Agent should not be called", func() {
					Expect(fakePolicyAgent.CheckCallCount()).To(Equal(0))
				})
			})

			Context("when request body is a bad yaml", func() {
				BeforeEach(func() {
					body := bytes.NewBuffer([]byte("a:\nb"))
					fakeRequest = httptest.NewRequest("PUT", "/something", body)
					fakeRequest.Header.Add("Content-type", "application/x-yaml")
				})

				It("should error", func() {
					Expect(checkErr).To(HaveOccurred())
					Expect(checkErr.Error()).To(Equal(`error converting YAML to JSON: yaml: line 3: could not find expected ':'`))
					Expect(result.Allowed).To(BeFalse())
				})
				It("Agent should not be called", func() {
					Expect(fakePolicyAgent.CheckCallCount()).To(Equal(0))
				})
			})

			Context("when every is ok", func() {
				BeforeEach(func() {
					fakeAccess.TeamRolesReturns(map[string][]string{
						"some-team": []string{"some-role"},
					})
					fakeAccess.ClaimsReturns(accessor.Claims{UserName: "some-user"})
				})

				Context("when the API endpoint is a team endpoint", func() {
					BeforeEach(func() {
						body := bytes.NewBuffer([]byte("a: b"))
						fakeRequest = httptest.NewRequest("PUT", "/something?:team_name=some-team", body)
						fakeRequest.Header.Add("Content-type", "application/x-yaml")
						fakeRequest.ParseForm()
					})

					It("should not error", func() {
						Expect(checkErr).ToNot(HaveOccurred())
					})
					It("Agent should be called", func() {
						Expect(fakePolicyAgent.CheckCallCount()).To(Equal(1))
					})
					It("Agent should take correct input", func() {
						Expect(fakePolicyAgent.CheckArgsForCall(0)).To(Equal(policy.PolicyCheckInput{
							Service:        "concourse",
							ClusterName:    "some-cluster",
							ClusterVersion: "some-version",
							HttpMethod:     "PUT",
							Action:         "some-action",
							User:           "some-user",
							Team:           "some-team",
							Roles:          []string{"some-role"},
							Data:           map[string]interface{}{"a": "b"},
						}))
					})

					It("request body should still be readable", func() {
						body, err := ioutil.ReadAll(fakeRequest.Body)
						Expect(err).ToNot(HaveOccurred())
						Expect(body).To(Equal([]byte("a: b")))
					})

					Context("when Agent says pass", func() {
						BeforeEach(func() {
							fakePolicyAgent.CheckReturns(policy.PassedPolicyCheck(), nil)
						})

						It("it should pass", func() {
							Expect(checkErr).ToNot(HaveOccurred())
							Expect(result.Allowed).To(BeTrue())
						})
					})

					Context("when Agent says not-pass", func() {
						BeforeEach(func() {
							fakePolicyAgent.CheckReturns(policy.PolicyCheckOutput{
								Allowed: false,
								Reasons: []string{"a policy says you can't do that"},
							}, nil)
						})

						It("should not pass", func() {
							Expect(checkErr).ToNot(HaveOccurred())
							Expect(result.Allowed).To(BeFalse())
							Expect(result.Reasons).To(ConsistOf("a policy says you can't do that"))
						})
					})

					Context("when Agent says error", func() {
						BeforeEach(func() {
							fakePolicyAgent.CheckReturns(policy.FailedPolicyCheck(), errors.New("some-error"))
						})

						It("should not pass", func() {
							Expect(checkErr).To(HaveOccurred())
							Expect(checkErr.Error()).To(Equal("some-error"))
							Expect(result.Allowed).To(BeFalse())
						})
					})
				})

				Context("when the API endpoint is a pipeline endpoint", func() {
					BeforeEach(func() {
						pipeline := new(dbfakes.FakePipeline)
						pipeline.TeamNameReturns("some-team")
						pipeline.NameReturns("some-pipeline")
						ctx := context.WithValue(context.Background(), auth.PipelineContextKey, pipeline)

						body := bytes.NewBuffer([]byte("a: b"))
						fakeRequest = httptest.NewRequest("PUT", "/something", body)
						fakeRequest.Header.Add("Content-type", "application/x-yaml")
						fakeRequest = fakeRequest.WithContext(ctx)
					})

					It("Agent should take correct input", func() {
						Expect(fakePolicyAgent.CheckArgsForCall(0)).To(Equal(policy.PolicyCheckInput{
							Service:        "concourse",
							ClusterName:    "some-cluster",
							ClusterVersion: "some-version",
							HttpMethod:     "PUT",
							Action:         "some-action",
							User:           "some-user",
							Team:           "some-team",
							Pipeline:       "some-pipeline",
							Roles:          []string{"some-role"},
							Data:           map[string]interface{}{"a": "b"},
						}))
					})
				})
			})
		})
	})
})
