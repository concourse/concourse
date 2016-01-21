package acceptance_test

import (
	"fmt"
	"net/http"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Auth", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var atcPort uint16

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(logger, dbConn, bus)

		err := sqlDB.DeleteTeamByName(atc.DefaultPipelineName)
		Expect(err).NotTo(HaveOccurred())
		team, err := sqlDB.SaveTeam(db.Team{Name: atc.DefaultTeamName})
		Expect(err).NotTo(HaveOccurred())

		_, _, err = sqlDB.SaveConfig(team.Name, atc.DefaultPipelineName, atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("GitHub Auth", func() {
		BeforeEach(func() {
			atcProcess, atcPort = startATC(atcBin, 1, false, GITHUB_AUTH)
		})

		It("forces a redirect to /login", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", atcPort), nil)
			resp, err := http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.Path).To(Equal("/login"))

			team, _, err := sqlDB.GetTeamByName(atc.DefaultTeamName)
			Expect(err).NotTo(HaveOccurred())
			Expect(team.ClientID).To(Equal("admin"))
			Expect(team.ClientSecret).To(Equal("password"))
			Expect(team.Organizations).To(Equal([]string{"myorg"}))
		})
	})

	Describe("Basic Auth", func() {
		BeforeEach(func() {
			atcProcess, atcPort = startATC(atcBin, 1, false, BASIC_AUTH)
		})

		It("forces a redirect to /login", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", atcPort), nil)
			resp, err := http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.Path).To(Equal("/login"))
		})

		It("logs in with Basic Auth and allows access", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", atcPort), nil)
			request.SetBasicAuth("admin", "password")
			resp, err := http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.Path).To(Equal("/"))
		})
	})

	Describe("No authentication via development mode", func() {
		BeforeEach(func() {
			atcProcess, atcPort = startATC(atcBin, 1, false, DEVELOPMENT_MODE)
		})

		It("logs in without authentication", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", atcPort), nil)
			resp, err := http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Request.URL.Path).To(Equal("/"))
		})
	})
})
