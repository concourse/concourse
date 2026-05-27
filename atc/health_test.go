package atc_test

import (
	"encoding/json"
	"time"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Health", func() {
	Describe("JSON marshaling", func() {
		It("marshals Health struct correctly", func() {
			timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
			health := atc.Health{
				Status:    atc.HealthStatusHealthy,
				Version:   "1.2.3",
				Timestamp: timestamp,
				Details: atc.HealthDetails{
					Database: atc.HealthStatus{
						Status: atc.HealthStatusHealthy,
					},
					WorkerHealth: atc.HealthStatus{
						Status:  atc.HealthStatusDegraded,
						Message: "Low worker count",
					},
				},
			}

			jsonBytes, err := json.Marshal(health)
			Expect(err).NotTo(HaveOccurred())

			var unmarshaled atc.Health
			err = json.Unmarshal(jsonBytes, &unmarshaled)
			Expect(err).NotTo(HaveOccurred())

			Expect(unmarshaled.Status).To(Equal(atc.HealthStatusHealthy))
			Expect(unmarshaled.Version).To(Equal("1.2.3"))
			Expect(unmarshaled.Timestamp).To(Equal(timestamp))
			Expect(unmarshaled.Details.Database.Status).To(Equal(atc.HealthStatusHealthy))
			Expect(unmarshaled.Details.WorkerHealth.Status).To(Equal(atc.HealthStatusDegraded))
			Expect(unmarshaled.Details.WorkerHealth.Message).To(Equal("Low worker count"))
		})

		It("handles empty messages correctly", func() {
			health := atc.Health{
				Status:    atc.HealthStatusUnhealthy,
				Timestamp: time.Now(),
				Details: atc.HealthDetails{
					Database: atc.HealthStatus{
						Status:  atc.HealthStatusUnhealthy,
						Message: "Connection failed",
					},
					WorkerHealth: atc.HealthStatus{
						Status: atc.HealthStatusHealthy,
					},
				},
			}

			jsonBytes, err := json.Marshal(health)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(jsonBytes)).To(ContainSubstring(`"message":"Connection failed"`))
		})
	})

	Describe("Health status constants", func() {
		It("has correct constant values", func() {
			Expect(atc.HealthStatusHealthy).To(Equal("healthy"))
			Expect(atc.HealthStatusUnhealthy).To(Equal("unhealthy"))
			Expect(atc.HealthStatusDegraded).To(Equal("degraded"))
		})
	})
})

