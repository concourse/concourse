package api_test

import (
	"bytes"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
)

var _ = Describe("Log Level API", func() {
	Describe("PUT /api/v1/log-level", func() {
		var (
			logLevelPayload string

			response *http.Response
		)

		BeforeEach(func() {
			logLevelPayload = ""
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", server.URL+"/api/v1/log-level", bytes.NewBufferString(logLevelPayload))
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			for x, y := range map[atc.LogLevel]lager.LogLevel{
				atc.LogLevelDebug: lager.DEBUG,
			} {
				atcLevel := x
				lagerLevel := y

				Context("when the level is "+string(atcLevel), func() {
					BeforeEach(func() {
						logLevelPayload = string(atcLevel)
					})

					It("sets the level to "+string(atcLevel), func() {
						Ω(sink.GetMinLevel()).Should(Equal(lagerLevel))
					})

					Describe("GET /api/v1/log-level", func() {
						var (
							getResponse *http.Response
						)

						JustBeforeEach(func() {
							req, err := http.NewRequest("GET", server.URL+"/api/v1/log-level", nil)
							Ω(err).ShouldNot(HaveOccurred())

							getResponse, err = client.Do(req)
							Ω(err).ShouldNot(HaveOccurred())
						})

						It("returns 200", func() {
							Ω(getResponse.StatusCode).Should(Equal(http.StatusOK))
						})

						It("returns the current log level", func() {
							Ω(ioutil.ReadAll(getResponse.Body)).Should(Equal([]byte(atcLevel)))
						})
					})
				})
			}

			Context("when the level is bogus", func() {
				BeforeEach(func() {
					logLevelPayload = "bogus"
				})

				It("returns Bad Request", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})
})
