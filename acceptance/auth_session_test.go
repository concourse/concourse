package acceptance_test

import (
	"net/http"
	"time"

	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/lib/pq"

	"encoding/json"
	"io/ioutil"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth Session", func() {
	var atcCommand *ATCCommand
	var dbListener *pq.Listener
	var page *agouti.Page

	BeforeEach(func() {
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(dbfakes.FakeConnector)
		retryableConn := &db.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := db.NewLockFactory(retryableConn)
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)

		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, NO_AUTH)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		page, err = agoutiDriver.NewPage()
		Expect(err).NotTo(HaveOccurred())

		dbngConn = dbng.Wrap(postgresRunner.Open())
		teamFactory := dbng.NewTeamFactory(dbngConn)
		defaultTeam, found, err := teamFactory.FindTeam(atc.DefaultTeamName)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue()) // created by postgresRunner

		_, _, err = defaultTeam.SavePipeline("main", atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "job-1",
				},
			},
		}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		Expect(page.Destroy()).To(Succeed())

		atcCommand.Stop()

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	homepage := func() string {
		return atcCommand.URL("")
	}

	login := func() {
		Expect(page.Navigate(homepage() + "/teams/main/login")).To(Succeed())
		Eventually(page.FindByButton("login")).Should(BeFound())
		Expect(page.FindByButton("login").Click()).To(Succeed())
		Eventually(page.Find("body")).Should(BeFound())
	}

	Context("when request does not contain CSRF token", func() {
		It("returns 400 Bad Request", func() {
			login()
			err := page.DeleteCookie("CSRF")
			Expect(err).NotTo(HaveOccurred())
			Expect(page.Navigate(atcCommand.URL("/teams/main/pipelines/main"))).To(Succeed())

			Eventually(page.Find("body")).Should(BeFound())
			Expect(page.Find("body")).To(HaveText("bad request"))
		})
	})

	Context("when request contains invalid CSRF token", func() {
		It("returns 400 Bad Request", func() {
			login()

			// Golang *Cookie notes that path and domain are optional
			// But for PhantomJS they are mandatory
			err := page.SetCookie(&http.Cookie{
				Name:   "CSRF",
				Value:  "invalid",
				Domain: "127.0.0.1",
				Path:   "/",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(page.Navigate(atcCommand.URL("/teams/main/pipelines/main"))).To(Succeed())

			Eventually(page.Find("body")).Should(BeFound())
			Expect(page.Find("body")).To(HaveText("bad request"))
		})
	})

	Context("when CSRF token and session token are not associated", func() {
		It("returns 401 Not Authorized", func() {
			login()
			cookies, err := page.GetCookies()
			Expect(err).NotTo(HaveOccurred())
			var firstCSRFToken *http.Cookie
			for _, cookie := range cookies {
				if cookie.Name == "CSRF" {
					firstCSRFToken = cookie
				}
			}
			Expect(firstCSRFToken).NotTo(BeNil())

			Login(page, homepage())

			err = page.SetCookie(firstCSRFToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(page.Navigate(atcCommand.URL("/teams/main/pipelines/main"))).To(Succeed())

			Eventually(page.Find("body")).Should(BeFound())
			Expect(page.Find("body")).To(HaveText("not authorized"))
		})
	})

	Context("when request contains valid CSRF with associated session token", func() {
		It("returns 200 OK", func() {
			login()
			Expect(page.Navigate(atcCommand.URL("/teams/main/pipelines/main"))).To(Succeed())
			Eventually(page.FindByLink("job-1")).Should(BeFound())
		})
	})

	Context("when request has authorization token in header", func() {
		var atcToken atc.AuthToken
		var client *http.Client

		BeforeEach(func() {
			request, err := http.NewRequest("GET", atcCommand.URL("/api/v1/teams/main/auth/token"), nil)
			client = &http.Client{
				Transport: &http.Transport{},
			}
			resp, err := client.Do(request)
			Expect(err).NotTo(HaveOccurred())

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			err = json.Unmarshal(body, &atcToken)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not require CSRF token", func() {
			request, err := http.NewRequest("GET", atcCommand.URL("/api/v1/teams/main/pipelines/main"), nil)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", atcToken.Type+" "+atcToken.Value)

			response, err := client.Do(request)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
