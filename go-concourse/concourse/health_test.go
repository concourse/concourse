package concourse_test

import (
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/go-concourse/concourse/internal"
)

var _ = Describe("GetHealth", func() {
	var healthResponse atc.Health

	BeforeEach(func() {
		healthResponse = atc.Health{
			Status:    atc.HealthStatusOK,
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Database:  atc.DatabaseHealth{Status: atc.HealthStatusHealthy},
			Workers: atc.WorkerHealth{
				Status:  string(atc.HealthStatusHealthy),
				Total:   3,
				Running: 3,
			},
			Components: []atc.ComponentHealth{
				{Name: atc.ComponentScheduler, Status: atc.HealthStatusHealthy, Paused: false},
			},
		}
	})

	Context("when the cluster is healthy (200)", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/health"),
					ghttp.RespondWithJSONEncoded(http.StatusOK, healthResponse),
				),
			)
		})

		It("returns the health response", func() {
			health, err := client.GetHealth()
			Expect(err).NotTo(HaveOccurred())
			Expect(health.Status).To(Equal(atc.HealthStatusOK))
			Expect(health.Database.Status).To(Equal(atc.HealthStatusHealthy))
			Expect(health.Workers.Total).To(Equal(3))
			Expect(health.Workers.Running).To(Equal(3))
			Expect(health.Components).To(HaveLen(1))
		})
	})

	Context("when the cluster is failing (503)", func() {
		BeforeEach(func() {
			healthResponse.Status = atc.HealthStatusFailing
			healthResponse.Database = atc.DatabaseHealth{
				Status: atc.HealthStatusUnhealthy,
				Error:  "database unreachable",
			}
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/health"),
					ghttp.RespondWithJSONEncoded(http.StatusServiceUnavailable, healthResponse),
				),
			)
		})

		It("still decodes the response body without returning an error", func() {
			health, err := client.GetHealth()
			Expect(err).NotTo(HaveOccurred())
			Expect(health.Status).To(Equal(atc.HealthStatusFailing))
			Expect(health.Database.Status).To(Equal(atc.HealthStatusUnhealthy))
			Expect(health.Database.Error).To(Equal("database unreachable"))
		})
	})

	Context("when the server returns an unexpected status code", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/api/v1/health"),
					ghttp.RespondWith(http.StatusInternalServerError, "internal error"),
				),
			)
		})

		It("returns an UnexpectedResponseError", func() {
			_, err := client.GetHealth()
			Expect(err).To(HaveOccurred())
			ure, ok := err.(internal.UnexpectedResponseError)
			Expect(ok).To(BeTrue())
			Expect(ure.StatusCode).To(Equal(http.StatusInternalServerError))
		})
	})
})
