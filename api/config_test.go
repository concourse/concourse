package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/turbine"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config API", func() {
	var (
		config atc.Config
	)

	BeforeEach(func() {
		config = atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"job-1", "job-2"},
					Resources: []string{"resource-1", "resource-2"},
				},
			},

			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
			},

			Jobs: atc.JobConfigs{
				{
					Name: "some-job",

					Public: true,

					BuildConfigPath: "some/config/path.yml",
					BuildConfig: turbine.Config{
						Image: "some-image",
					},

					Privileged: true,

					Serial: true,

					Inputs: []atc.InputConfig{
						{
							Name:     "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed: []string{"job-1", "job-2"},
						},
					},

					Outputs: []atc.OutputConfig{
						{
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							PerformOn: []atc.OutputCondition{"success", "failure"},
						},
					},
				},
			},
		}
	})

	Describe("GET /api/v1/config", func() {
		var (
			response *http.Response
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/config", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when the config can be loaded", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(config, nil)
			})

			It("returns 200", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns the config", func() {
				var returnedConfig atc.Config
				err := json.NewDecoder(response.Body).Decode(&returnedConfig)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(returnedConfig).Should(Equal(config))
			})
		})

		Context("when getting the config fails", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{}, errors.New("oh no!"))
			})

			It("returns 500", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("PUT /api/v1/config", func() {
		var (
			response *http.Response
		)

		JustBeforeEach(func() {
			payload, err := json.Marshal(config)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/api/v1/config", ioutil.NopCloser(bytes.NewBuffer(payload)))
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when the config is valid", func() {
			It("returns 200", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("saves it", func() {
				Ω(configDB.SaveConfigCallCount()).Should(Equal(1))
				Ω(configDB.SaveConfigArgsForCall(0)).Should(Equal(config))
			})

			Context("and saving it fails", func() {
				BeforeEach(func() {
					configDB.SaveConfigReturns(errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when the config is invalid", func() {
			BeforeEach(func() {
				configValidationErr = errors.New("totally invalid")
			})

			It("returns 400", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
			})

			It("returns the validation error in the response body", func() {
				Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("totally invalid")))
			})

			It("does not save it", func() {
				Ω(configDB.SaveConfigCallCount()).Should(BeZero())
			})
		})
	})
})
