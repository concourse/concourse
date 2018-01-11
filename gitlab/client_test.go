package gitlab_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/skymarshal/gitlab"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	gogitlab "github.com/xanzy/go-gitlab"
)

var _ = Describe("Client", func() {

	var (
		gitlabServer *ghttp.Server

		client gitlab.Client

		proxiedClient *http.Client
	)

	BeforeEach(func() {
		gitlabServer = ghttp.NewServer()

		client = gitlab.NewClient("http://gitlab.mydomain.com/api/v4")

		proxiedClient = &http.Client{
			Transport: proxiedTransport{gitlabServer},
		}
	})

	Describe("Groups", func() {
		Context("when listing groups succeeds", func() {
			BeforeEach(func() {
				gitlabServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v4/groups", "page=1"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogitlab.Group{
								{Name: "group-1"},
								{Name: "group-2"},
							},
							http.Header{
								"Link": []string{
									fmt.Sprintf(`<http://%s/api/v4/groups?page=2>; rel="next"`, gitlabServer.Addr()),
									fmt.Sprintf(`<http://%s/api/v4/groups?page=2>; rel="last"`, gitlabServer.Addr()),
								},
							},
						),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v4/groups", "page=2"),
						ghttp.RespondWithJSONEncoded(
							http.StatusOK,
							[]gogitlab.Group{
								{Name: "group-3"},
							},
							http.Header{
								"Link": []string{
									fmt.Sprintf(`<http://%s/api/v4/groups?page=1>; rel="first"`, gitlabServer.Addr()),
									fmt.Sprintf(`<http://%s/api/v4/groups?page=1>; rel="prev"`, gitlabServer.Addr()),
								},
							},
						),
					),
				)
			})

			It("returns the list of group names", func() {
				groups, err := client.Groups(proxiedClient)
				Expect(err).NotTo(HaveOccurred())
				Expect(groups).To(Equal([]string{"group-1", "group-2", "group-3"}))
			})
		})

		Context("when listing groups fails", func() {
			BeforeEach(func() {
				gitlabServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/api/v4/groups"),
						ghttp.RespondWith(http.StatusUnauthorized, ""),
					),
				)
			})

			It("returns an error", func() {
				_, err := client.Groups(proxiedClient)
				Expect(err).To(BeAssignableToTypeOf(&gogitlab.ErrorResponse{}))
			})
		})
	})
})

type proxiedTransport struct {
	proxy *ghttp.Server
}

func (t proxiedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := *req.URL
	newURL.Scheme = "http"
	newURL.Host = t.proxy.Addr()

	newReq := *req
	newReq.URL = &newURL

	return (&http.Transport{}).RoundTrip(&newReq)
}
