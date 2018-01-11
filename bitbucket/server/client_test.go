package server_test

import (
	"fmt"
	"github.com/concourse/skymarshal/bitbucket"
	"github.com/concourse/skymarshal/bitbucket/server"
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

		client = server.NewClient(bitbucketServer.URL())
	})

	Describe("CurrentUser", func() {
		Context("when getting the current user succeeds", func() {
			Context("when the X-Ausername header is set", func() {
				BeforeEach(func() {
					bitbucketServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/rest/api/1.0/users"),
							ghttp.RespondWith(http.StatusOK, "", http.Header{"X-Ausername": []string{"some-user"}}),
						),
					)
				})

				It("returns the user's name", func() {
					user, err := client.CurrentUser(httpClient)
					Expect(err).ToNot(HaveOccurred())
					Expect(user).To(Equal("some-user"))
				})
			})

			Context("when the X-Ausername header is not set", func() {
				BeforeEach(func() {
					bitbucketServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/rest/api/1.0/users"),
							ghttp.RespondWith(http.StatusOK, ""),
						),
					)
				})

				It("returns an empty string", func() {
					user, err := client.CurrentUser(httpClient)
					Expect(err).ToNot(HaveOccurred())
					Expect(user).To(BeEmpty())
				})
			})
		})

		Context("when getting the current user fails", func() {
			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/rest/api/1.0/users"),
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
						ghttp.VerifyRequest("GET", fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s", owner, repository)),
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
						ghttp.VerifyRequest("GET", fmt.Sprintf("/rest/api/1.0/projects/%s/repos/%s", owner, repository)),
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
		Context("when listing projects succeeds", func() {
			team := "some-project"
			key := "PRJ"

			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/rest/api/1.0/projects"),
						ghttp.RespondWith(http.StatusOK, fmt.Sprintf(`{"isLastPage":true,"values":[{"name":"%s","key":"%s"}]}`, team, key)),
					),
				)
			})

			It("returns the list of project keys", func() {
				projects, err := client.Projects(httpClient)
				Expect(err).ToNot(HaveOccurred())
				Expect(projects).To(Equal([]string{"PRJ"}))
			})
		})

		Context("when listing projects fails", func() {
			BeforeEach(func() {
				bitbucketServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/rest/api/1.0/projects"),
						ghttp.RespondWith(http.StatusUnauthorized, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := client.Projects(httpClient)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Teams", func() {
		It("returns nil", func() {
			teams, err := client.Teams(httpClient, "contributor")
			Expect(err).ToNot(HaveOccurred())
			Expect(teams).To(BeNil())
		})
	})
})
