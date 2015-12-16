package acceptance_test

import (
	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/sclevine/agouti"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Resource Pagination", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var pipelineDB db.PipelineDB
	var atcPort uint16

	BeforeEach(func() {
		dbLogger := lagertest.NewTestLogger("test")
		postgresRunner.Truncate()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)

		sqlDB = db.NewSQL(dbLogger, dbConn, bus)

		atcProcess, atcPort = startATC(atcBin, 1, true, BASIC_AUTH)

		_, err := dbConn.Query(`DELETE FROM teams WHERE name = 'main'`)
		Expect(err).NotTo(HaveOccurred())
		team, err := sqlDB.SaveTeam(db.Team{Name: atc.DefaultTeamName})
		Expect(err).NotTo(HaveOccurred())

		// job build data
		_, err = sqlDB.SaveConfig(team.Name, atc.DefaultPipelineName, atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "job-name",
					Plan: atc.PlanSequence{
						{
							Get: "resource-name",
						},
					},
				},
			},
			Resources: atc.ResourceConfigs{
				{Name: "resource-name"},
			},
		}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(dbLogger, dbConn, bus, sqlDB)

		var found bool
		pipelineDB, found, err = pipelineDBFactory.BuildDefault()
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("pages", func() {
		var page *agouti.Page

		BeforeEach(func() {
			var err error
			page, err = agoutiDriver.NewPage()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(page.Destroy()).To(Succeed())
		})

		homepage := func() string {
			return fmt.Sprintf("http://127.0.0.1:%d/pipelines/%s", atcPort, atc.DefaultPipelineName)
		}

		withPath := func(path string) string {
			return urljoiner.Join(homepage(), path)
		}

		Context("with more than 100 resource versions", func() {
			BeforeEach(func() {
				resourceConfig := atc.ResourceConfig{
					Name:   "resource-name",
					Type:   "some-type",
					Source: atc.Source{},
				}

				var resourceVersions []atc.Version
				for i := 0; i < 104; i++ {
					resourceVersions = append(resourceVersions, atc.Version{"version": strconv.Itoa(i)})
				}

				err := pipelineDB.SaveResourceVersions(resourceConfig, resourceVersions)
				Expect(err).NotTo(HaveOccurred())
			})

			It("there is pagination", func() {
				// homepage -> resource detail
				Expect(page.Navigate(homepage())).To(Succeed())
				Eventually(page.FindByLink("resource-name")).Should(BeFound())
				Expect(page.FindByLink("resource-name").Click()).To(Succeed())

				// resource detail -> paused resource detail
				Eventually(page).Should(HaveURL(withPath("/resources/resource-name")))
				Expect(page.Find("h1")).To(HaveText("resource-name"))
				Expect(page.All(".pagination").Count()).Should(Equal(2))
				Expect(page.Find(".resource-versions")).Should(BeFound())
				Expect(page.All(".resource-versions li").Count()).Should(Equal(100))

				Expect(page.First(".pagination .disabled .fa-arrow-left")).Should(BeFound())
				Expect(page.First(".pagination .fa-arrow-right").Click()).To(Succeed())
				Eventually(page.All(".resource-versions li").Count).Should(Equal(4))

				Expect(page.First(".pagination .disabled .fa-arrow-right")).Should(BeFound())
				Expect(page.First(".pagination .fa-arrow-left").Click()).To(Succeed())
				Eventually(page.All(".resource-versions li").Count).Should(Equal(100))
			})
		})

		Context("with less than 100 resource versions", func() {
			BeforeEach(func() {
				resourceConfig := atc.ResourceConfig{
					Name:   "resource-name",
					Type:   "some-type",
					Source: atc.Source{},
				}

				var resourceVersions []atc.Version
				for i := 0; i < 99; i++ {
					resourceVersions = append(resourceVersions, atc.Version{"version": strconv.Itoa(i)})
				}

				err := pipelineDB.SaveResourceVersions(resourceConfig, resourceVersions)
				Expect(err).NotTo(HaveOccurred())
			})

			It("shows disabled pagination buttons", func() {
				// homepage -> resource detail
				Expect(page.Navigate(homepage())).To(Succeed())
				Eventually(page.FindByLink("resource-name")).Should(BeFound())
				Expect(page.FindByLink("resource-name").Click()).To(Succeed())

				// resource detail -> paused resource detail
				Eventually(page).Should(HaveURL(withPath("/resources/resource-name")))
				Expect(page.Find("h1")).To(HaveText("resource-name"))
				Expect(page.First(".pagination .disabled .fa-arrow-left")).Should(BeFound())
				Expect(page.First(".pagination .disabled .fa-arrow-right")).Should(BeFound())
				Expect(page.Find(".resource-versions")).Should(BeFound())
				Expect(page.All(".resource-versions li").Count()).Should(Equal(99))
			})
		})
	})
})
