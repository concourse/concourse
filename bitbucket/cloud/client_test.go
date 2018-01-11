package cloud_test

import (
	"fmt"
	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/bitbucket/cloud"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"net/http"
)

var _ = Describe("Client", func() {
	var (
		bitbucketServer *ghttp.Server

		client bitbucket.Client

		httpClient *http.Client
	)

	BeforeEach(func() {
		bitbucketServer = ghttp.NewServer()

		client = cloud.NewClient(bitbucketServer.URL())
	})

	Describe("CurrentUser", func() {
		Context("when getting the current user succeeds", func() {
			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/2.0/user"),
						ghttp.RespondWith(http.StatusOK, `{"username":"some-user"}`),
					),
				)
			})

			It("returns the user's name", func() {
				user, err := client.CurrentUser(httpClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(user).To(Equal("some-user"))
			})
		})

		Context("when getting the current user fails", func() {
			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/2.0/user"),
						ghttp.RespondWith(http.StatusUnauthorized, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := client.CurrentUser(httpClient)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Repository", func() {
		owner := "some-user"
		repository := "some-repository"

		Context("when getting the repository succeeds", func() {
			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/2.0/repositories/%s/%s", owner, repository)),
						ghttp.RespondWith(http.StatusOK, fmt.Sprintf(`{"full_name":"%s/%s","name":"%s"}`, owner, repository, repository)),
					),
				)
			})

			It("returns true", func() {
				found, err := client.Repository(httpClient, owner, repository)
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when getting the repository fails", func() {
			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", fmt.Sprintf("/2.0/repositories/%s/%s", owner, repository)),
						ghttp.RespondWith(http.StatusUnauthorized, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := client.Repository(httpClient, owner, repository)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Projects", func() {
		It("returns nil", func() {
			projects, err := client.Projects(httpClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(projects).To(BeNil())
		})
	})

	Describe("Teams", func() {
		role := "contributor"

		Context("when listing teams succeeds", func() {
			team := "some-team"

			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/2.0/teams", fmt.Sprintf("role=%s", role)),
						ghttp.RespondWith(http.StatusOK, fmt.Sprintf(`{"isLastPage":true,"values":[{"username":"%s"}]}`, team)),
					),
				)
			})

			It("returns the list of team names", func() {
				teams, err := client.Teams(httpClient, role)
				Expect(err).ToNot(HaveOccurred())
				Expect(teams).To(Equal([]string{"some-team"}))
			})
		})

		Context("when listing teams fails", func() {
			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/2.0/teams", fmt.Sprintf("role=%s", role)),
						ghttp.RespondWith(http.StatusUnauthorized, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := client.Teams(httpClient, role)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
