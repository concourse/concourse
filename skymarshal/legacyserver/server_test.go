package legacyserver_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Legacy API", func() {
	var err error
	var request *http.Request
	var response *http.Response

	BeforeEach(func() {
		client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{},
		}
	})

	Context("GET /login", func() {

		Context("when given no query params", func() {

			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL+"/login", nil)
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return status 302", func() {
				Expect(response.StatusCode).To(Equal(http.StatusFound))

				url, err := response.Location()
				Expect(err).NotTo(HaveOccurred())
				Expect(url.Path).To(Equal("/sky/login"))
			})

		})

		Context("when given fly_port query param", func() {

			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL+"/login?fly_port=1234", nil)
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add redirect_uri", func() {
				Expect(response.StatusCode).To(Equal(http.StatusFound))

				url, err := response.Location()
				Expect(err).NotTo(HaveOccurred())
				Expect(url.Path).To(Equal("/sky/login"))
				Expect(url.Query()["redirect_uri"]).To(Equal(
					[]string{"/fly_success?fly_port=1234"},
				))
			})
		})

		Context("when given other query param", func() {

			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL+"/login?foo=bar", nil)
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should leave query params in the redirect", func() {
				Expect(response.StatusCode).To(Equal(http.StatusFound))

				url, err := response.Location()
				Expect(err).NotTo(HaveOccurred())
				Expect(url.Path).To(Equal("/sky/login"))
				Expect(url.Query()["foo"]).To(Equal(
					[]string{"bar"},
				))
			})
		})
	})

	Context("GET /logout", func() {

		BeforeEach(func() {
			request, err = http.NewRequest("GET", server.URL+"/logout", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return status 301", func() {
			Expect(response.StatusCode).To(Equal(http.StatusMovedPermanently))

			url, err := response.Location()
			Expect(err).NotTo(HaveOccurred())
			Expect(url.Path).To(Equal("/sky/logout"))
		})
	})

	Context("GET /auth/:provider/callback", func() {

		BeforeEach(func() {
			request, err = http.NewRequest("GET", server.URL+"/auth/github/callback?code=1234567890&state=asdfghjkl", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return status 301", func() {
			Expect(response.StatusCode).To(Equal(http.StatusMovedPermanently))

			url, err := response.Location()
			Expect(err).NotTo(HaveOccurred())
			Expect(url.Path).To(Equal("/sky/issuer/callback"))
			Expect(url.RawQuery).To(Equal("code=1234567890&state=asdfghjkl"))
		})
	})
})
