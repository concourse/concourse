package policy_test

import (
	"bytes"
	"errors"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/policy/policyfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Policy checker", func() {

	var (
		checker policy.Checker
		filter  policy.Filter
		err     error
	)

	BeforeEach(func() {
		filter = policy.Filter{
			HttpMethods:   []string{"POST,PUT"},
			Actions:       []string{"do_1,do_2"},
			ActionsToSkip: []string{"skip_1,skip_2"},
		}

		fakeAgent = new(policyfakes.FakeAgent)
		fakeAgentFactory.NewAgentReturns(fakeAgent, nil)
	})

	JustBeforeEach(func() {
		checker, err = policy.Initialize(testLogger, "some-cluster", "some-version", filter)
	})

	// fakeAgent is configured in BeforeSuite.
	Context("Initialize", func() {
		It("should return a checker", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(checker).ToNot(BeNil())
		})

		Context("Checker", func() {
			Context("CheckHttpApi", func() {
				var (
					fakeAccessor *accessorfakes.FakeAccess
					fakeRequest  *http.Request
					action       string
					pass         bool
				)

				BeforeEach(func() {
					fakeAccessor = new(accessorfakes.FakeAccess)
					fakeAccessor.UserNameReturns("some-user")
				})

				JustBeforeEach(func() {
					pass, err = checker.CheckHttpApi(action, fakeAccessor, fakeRequest)
				})

				Context("when it's system action", func() {
					BeforeEach(func() {
						action = "some-action"
						fakeAccessor.IsSystemReturns(true)
					})

					It("should pass", func() {
						Expect(err).ToNot(HaveOccurred())
						Expect(pass).To(BeTrue())
					})

					It("agent should not be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(0))
					})
				})

				Context("action skip_1", func() {
					BeforeEach(func() {
						action = "skip_1"
					})

					It("should pass", func() {
						Expect(err).ToNot(HaveOccurred())
						Expect(pass).To(BeTrue())
					})

					It("agent should not be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(0))
					})
				})

				Context("HTTP GET method", func() {
					BeforeEach(func() {
						action = "some-action"
						fakeRequest = httptest.NewRequest("GET", "/something", nil)
					})

					It("should pass", func() {
						Expect(err).ToNot(HaveOccurred())
						Expect(pass).To(BeTrue())
					})

					It("agent should not be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(0))
					})
				})

				Context("HTTP GET method and action do_1", func() {
					BeforeEach(func() {
						action = "do_1"
						fakeRequest = httptest.NewRequest("GET", "/something", nil)
					})

					It("should go through agent", func() {
						Expect(err).ToNot(HaveOccurred())
						Expect(fakeAgent.CheckCallCount()).To(Equal(1))
					})
				})

				Context("when request body is a bad json", func() {
					BeforeEach(func() {
						action = "some-action"
						body := bytes.NewBuffer([]byte("hello"))
						fakeRequest = httptest.NewRequest("PUT", "/something", body)
						fakeRequest.Header.Add("Content-type", "application/json")
					})

					It("should error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(`invalid character 'h' looking for beginning of value`))
						Expect(pass).To(BeFalse())
					})

					It("agent should not be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(0))
					})
				})

				Context("when request body is a bad yaml", func() {
					BeforeEach(func() {
						action = "some-action"
						body := bytes.NewBuffer([]byte("a:\nb"))
						fakeRequest = httptest.NewRequest("PUT", "/something", body)
						fakeRequest.Header.Add("Content-type", "application/x-yaml")
					})

					It("should error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(`error converting YAML to JSON: yaml: line 3: could not find expected ':'`))
						Expect(pass).To(BeFalse())
					})

					It("agent should not be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(0))
					})
				})

				Context("when every is ok", func() {
					BeforeEach(func() {
						action = "some-action"
						body := bytes.NewBuffer([]byte("a: b"))
						fakeRequest = httptest.NewRequest("PUT", "/something?:team_name=some-team&:pipeline_name=some-pipeline", body)
						fakeRequest.Header.Add("Content-type", "application/x-yaml")
						fakeRequest.ParseForm()
					})

					It("should not error", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("agent should be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(1))
					})

					It("agent should take correct input", func() {
						Expect(fakeAgent.CheckArgsForCall(0)).To(Equal(policy.PolicyCheckInput{
							Service:        "concourse",
							ClusterName:    "some-cluster",
							ClusterVersion: "some-version",
							HttpMethod:     "PUT",
							Action:         action,
							User:           "some-user",
							Team:           "some-team",
							Pipeline:       "some-pipeline",
							Data:           map[string]interface{}{"a": "b"},
						}))
					})

					It("request body should still be readable", func() {
						body, err := ioutil.ReadAll(fakeRequest.Body)
						Expect(err).ToNot(HaveOccurred())
						Expect(body).To(Equal([]byte("a: b")))
					})

					Context("when agent says pass", func() {
						BeforeEach(func() {
							fakeAgent.CheckReturns(true, nil)
						})

						It("it should pass", func() {
							Expect(err).ToNot(HaveOccurred())
							Expect(pass).To(BeTrue())
						})
					})

					Context("when agent says not-pass", func() {
						BeforeEach(func() {
							fakeAgent.CheckReturns(false, nil)
						})

						It("should not pass", func() {
							Expect(err).ToNot(HaveOccurred())
							Expect(pass).To(BeFalse())
						})
					})

					Context("when agent says error", func() {
						BeforeEach(func() {
							fakeAgent.CheckReturns(false, errors.New("some-error"))
						})

						It("should not pass", func() {
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(Equal("some-error"))
							Expect(pass).To(BeFalse())
						})
					})
				})
			})

			Context("CheckUsingImage", func() {
				var (
					fakeBuild  *dbfakes.FakeBuild
					pass       bool
				)

				BeforeEach(func() {
					fakeBuild = new(dbfakes.FakeBuild)
				})

				JustBeforeEach(func() {
					pass, err = checker.CheckUsingImage("some-team", "some-pipeline", "task", atc.Source{"repository": "some-image"})
				})

				Context("when UsingImage is in skip list", func() {
					BeforeEach(func() {
						filter = policy.Filter{
							HttpMethods:   []string{"POST,PUT"},
							Actions:       []string{"do_1,do_2"},
							ActionsToSkip: []string{policy.ActionUsingImage},
						}
					})

					It("should pass", func() {
						Expect(err).ToNot(HaveOccurred())
						Expect(pass).To(BeTrue())
					})

					It("agent should not be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(0))
					})
				})

				Context("when RunTask is not in skip list", func() {
					BeforeEach(func() {
						filter = policy.Filter{
							HttpMethods:   []string{"POST,PUT"},
							Actions:       []string{"do_1,do_2"},
							ActionsToSkip: []string{},
						}

						fakeBuild.TeamNameReturns("some-team")
						fakeBuild.PipelineNameReturns("some-pipeline")
					})

					It("should not error", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("agent should be called", func() {
						Expect(fakeAgent.CheckCallCount()).To(Equal(1))
					})

					It("agent should take correct input", func() {
						Expect(fakeAgent.CheckArgsForCall(0)).To(Equal(policy.PolicyCheckInput{
							Service:        "concourse",
							ClusterName:    "some-cluster",
							ClusterVersion: "some-version",
							Action:         policy.ActionUsingImage,
							Team:           "some-team",
							Pipeline:       "some-pipeline",
							Data: map[string]string{
								"repository": "some-image",
								"step":       "task",
								"tag":        "latest",
							},
						}))
					})

					Context("when agent says pass", func() {
						BeforeEach(func() {
							fakeAgent.CheckReturns(true, nil)
						})

						It("it should pass", func() {
							Expect(err).ToNot(HaveOccurred())
							Expect(pass).To(BeTrue())
						})
					})

					Context("when agent says not-pass", func() {
						BeforeEach(func() {
							fakeAgent.CheckReturns(false, nil)
						})

						It("should not pass", func() {
							Expect(err).ToNot(HaveOccurred())
							Expect(pass).To(BeFalse())
						})
					})

					Context("when agent says error", func() {
						BeforeEach(func() {
							fakeAgent.CheckReturns(false, errors.New("some-error"))
						})

						It("should not pass", func() {
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(Equal("some-error"))
							Expect(pass).To(BeFalse())
						})
					})
				})
			})
		})
	})
})
