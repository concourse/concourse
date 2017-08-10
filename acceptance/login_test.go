package acceptance_test

import (
	"fmt"
	"net/url"

	"github.com/sclevine/agouti"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Logging In", func() {
	var atcCommand *ATCCommand
	var pipelineName string
	var pipeline db.Pipeline

	BeforeEach(func() {
		var err error
		pipelineName = atc.DefaultPipelineName

		pipeline, _, err = defaultTeam.SavePipeline(pipelineName, atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "job-name"},
			},
		}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, BASIC_AUTH)
		err = atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		atcCommand.Stop()
	})

	homepage := func() string {
		return atcCommand.URL("")
	}

	Describe("logging in via the UI", func() {
		Context("when user is not logged in", func() {
			var page *agouti.Page

			BeforeEach(func() {
				var err error
				page, err = agoutiDriver.NewPage()
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(page.Destroy()).To(Succeed())
			})

			Describe("log in with bad credentials", func() {
				BeforeEach(func() {
					Expect(page.Navigate(homepage() + "/teams/main/login")).To(Succeed())
					FillLoginFormWithCredentials(page, "some-user", "bad-password")
				})

				It("shows an error message", func() {
					Expect(page.FindByButton("login").Click()).To(Succeed())
					Eventually(page.FindByClass("login-error")).Should(BeVisible())
				})
			})

			Describe("after the user logs in", func() {
				BeforeEach(func() {
					Login(page, homepage())
				})

				It("should display the pipelines the user has access to in the sidebar", func() {
					Expect(page.FindByClass("sidebar-toggle").Click()).To(Succeed())
					Eventually(page.FindByLink("main")).Should(BeVisible())
				})

				It("should no longer display the login link", func() {
					Eventually(page.FindByLink("login")).ShouldNot(BeFound())
				})
			})

			Context("navigating to a team specific page", func() {
				BeforeEach(func() {
					Expect(page.Navigate(atcCommand.URL("/teams/main/pipelines/main"))).To(Succeed())
				})

				It("forces a redirect to /teams/main/login with a redirect query param", func() {
					Eventually(page).Should(HaveURL(atcCommand.URL(fmt.Sprintf("/teams/main/login?redirect=%s", url.QueryEscape("/teams/main/pipelines/main")))))
				})
			})

			Context("when a build exists for an authenticated team", func() {
				var buildPath string

				BeforeEach(func() {
					// job build data
					job, found, err := pipeline.Job("job-name")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())

					build, err := job.CreateBuild()
					Expect(err).NotTo(HaveOccurred())
					buildPath = fmt.Sprintf("/builds/%d", build.ID())
				})

				Context("navigating to a team specific page that exists", func() {
					BeforeEach(func() {
						Expect(page.Navigate(atcCommand.URL(buildPath))).To(Succeed())
					})

					It("forces a redirect to /login", func() {
						Eventually(page).Should(HaveURL(atcCommand.URL(fmt.Sprintf("/login?redirect=%s", url.QueryEscape(buildPath)))))
					})

					It("redirects back to the build page when user logs in", func() {
						Eventually(page.FindByLink(atc.DefaultTeamName)).Should(BeFound())
						Expect(page.FindByLink(atc.DefaultTeamName).Click()).To(Succeed())
						FillLoginFormAndSubmit(page)
						Eventually(page).Should(HaveURL(atcCommand.URL(buildPath)))
					})
				})
			})
		})
	})
})
