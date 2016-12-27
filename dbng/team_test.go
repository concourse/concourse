package dbng_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Team", func() {
	Describe("SaveWorker", func() {
		var (
			team        dbng.Team
			otherTeam   dbng.Team
			teamFactory dbng.TeamFactory

			atcWorker atc.Worker
		)

		BeforeEach(func() {
			var err error
			teamFactory = dbng.NewTeamFactory(dbConn)
			team, err = teamFactory.CreateTeam("team")
			Expect(err).NotTo(HaveOccurred())
			otherTeam, err = teamFactory.CreateTeam("otherTeam")
			Expect(err).NotTo(HaveOccurred())

			atcWorker = atc.Worker{
				GardenAddr:       "some-garden-addr",
				BaggageclaimURL:  "some-bc-url",
				HTTPProxyURL:     "some-http-proxy-url",
				HTTPSProxyURL:    "some-https-proxy-url",
				NoProxy:          "some-no-proxy",
				ActiveContainers: 140,
				ResourceTypes: []atc.WorkerResourceType{
					{
						Type:    "some-resource-type",
						Image:   "some-image",
						Version: "some-version",
					},
					{
						Type:    "other-resource-type",
						Image:   "other-image",
						Version: "other-version",
					},
				},
				Platform:  "some-platform",
				Tags:      atc.Tags{"some", "tags"},
				Name:      "some-name",
				StartTime: 55,
			}
		})

		Context("the worker already exists", func() {
			Context("the worker is not in stalled state", func() {
				Context("the team_id of the new worker is the same", func() {
					BeforeEach(func() {
						_, err := team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
					})
					It("overwrites all the data", func() {
						atcWorker.GardenAddr = "new-garden-addr"
						savedWorker, err := team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
						Expect(savedWorker.Name).To(Equal("some-name"))
						Expect(*savedWorker.GardenAddr).To(Equal("new-garden-addr"))
						Expect(savedWorker.State).To(Equal(dbng.WorkerStateRunning))
					})
				})
				Context("the team_id of the new worker is different", func() {
					BeforeEach(func() {
						_, err := otherTeam.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).NotTo(HaveOccurred())
					})
					It("errors", func() {
						_, err := team.SaveWorker(atcWorker, 5*time.Minute)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})
	})
})
