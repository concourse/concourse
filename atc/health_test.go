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
				Status:    atc.HealthStatusOK,
				Timestamp: timestamp,
				Database:  atc.DatabaseHealth{Status: atc.HealthStatusHealthy},
				Workers:   atc.WorkerHealth{Status: atc.HealthStatusHealthy, Total: 3, Running: 3},
			}

			jsonBytes, err := json.Marshal(health)
			Expect(err).NotTo(HaveOccurred())

			var unmarshaled atc.Health
			Expect(json.Unmarshal(jsonBytes, &unmarshaled)).To(Succeed())
			Expect(unmarshaled.Status).To(Equal(atc.HealthStatusOK))
			Expect(unmarshaled.Timestamp).To(Equal(timestamp))
			Expect(unmarshaled.Database.Status).To(Equal(atc.HealthStatusHealthy))
			Expect(unmarshaled.Workers.Status).To(Equal(atc.HealthStatusHealthy))
			Expect(unmarshaled.Workers.Total).To(Equal(3))
			Expect(unmarshaled.Workers.Running).To(Equal(3))
		})

		It("includes the error field when database is unhealthy", func() {
			health := atc.Health{
				Status:    atc.HealthStatusFailing,
				Timestamp: time.Now(),
				Database:  atc.DatabaseHealth{Status: atc.HealthStatusUnhealthy, Error: "connection refused"},
				Workers:   atc.WorkerHealth{Status: atc.HealthStatusHealthy},
			}

			jsonBytes, err := json.Marshal(health)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(jsonBytes)).To(ContainSubstring(`"error":"connection refused"`))
		})

		It("omits the error field when database is healthy", func() {
			health := atc.Health{
				Status:    atc.HealthStatusOK,
				Timestamp: time.Now(),
				Database:  atc.DatabaseHealth{Status: atc.HealthStatusHealthy},
				Workers:   atc.WorkerHealth{Status: atc.HealthStatusHealthy, Total: 1, Running: 1},
			}

			jsonBytes, err := json.Marshal(health)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(jsonBytes)).NotTo(ContainSubstring(`"error"`))
		})

		It("marshals Health struct with components correctly", func() {
			timestamp := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
			lastRan := time.Date(2023, 1, 1, 11, 55, 0, 0, time.UTC)
			health := atc.Health{
				Status:    atc.HealthStatusDegraded,
				Timestamp: timestamp,
				Database:  atc.DatabaseHealth{Status: atc.HealthStatusHealthy},
				Workers:   atc.WorkerHealth{Status: atc.HealthStatusHealthy, Total: 1, Running: 1},
				Components: []atc.ComponentHealth{
					{Name: atc.ComponentScheduler, Status: atc.HealthStatusUnhealthy, Paused: false, LastRan: lastRan, Stale: true},
					{Name: atc.ComponentCollectorVolumes, Status: atc.HealthStatusUnhealthy, Paused: false, LastRan: lastRan, Stale: true},
				},
			}

			jsonBytes, err := json.Marshal(health)
			Expect(err).NotTo(HaveOccurred())

			var unmarshaled atc.Health
			Expect(json.Unmarshal(jsonBytes, &unmarshaled)).To(Succeed())
			Expect(unmarshaled.Components).To(HaveLen(2))
			Expect(unmarshaled.Components[0].Name).To(Equal(atc.ComponentScheduler))
			Expect(unmarshaled.Components[0].Stale).To(BeTrue())
			Expect(unmarshaled.Components[1].Name).To(Equal(atc.ComponentCollectorVolumes))
		})

		It("marshals an empty components list as an empty array", func() {
			health := atc.Health{
				Status:     atc.HealthStatusOK,
				Timestamp:  time.Now(),
				Database:   atc.DatabaseHealth{Status: atc.HealthStatusHealthy},
				Workers:    atc.WorkerHealth{Status: atc.HealthStatusHealthy, Total: 1, Running: 1},
				Components: []atc.ComponentHealth{},
			}

			jsonBytes, err := json.Marshal(health)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(jsonBytes)).To(ContainSubstring(`"components":[]`))
		})
	})

	Describe("Health status constants", func() {
		It("has correct overall status values", func() {
			Expect(atc.HealthStatusOK).To(Equal("ok"))
			Expect(atc.HealthStatusDegraded).To(Equal("degraded"))
			Expect(atc.HealthStatusFailing).To(Equal("failing"))
		})

		It("has correct subsystem status values", func() {
			Expect(atc.HealthStatusHealthy).To(Equal("healthy"))
			Expect(atc.HealthStatusUnhealthy).To(Equal("unhealthy"))
		})
	})
})
