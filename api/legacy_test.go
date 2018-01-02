package api_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
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

	Context("GET /api/v1/teams/main/auth/methods", func() {

		BeforeEach(func() {
			request, err = http.NewRequest("GET", server.URL+"/api/v1/teams/main/auth/methods", nil)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Content-Type", "application/json")

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return status 301", func() {
			Expect(response.StatusCode).To(Equal(http.StatusMovedPermanently))

			url, err := response.Location()
			Expect(err).NotTo(HaveOccurred())
			Expect(url.Path).To(Equal("/auth/list_methods"))
		})
	})

	Context("GET /api/v1/teams/main/auth/token", func() {

		BeforeEach(func() {
			request, err = http.NewRequest("GET", server.URL+"/api/v1/teams/main/auth/token", nil)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Content-Type", "application/json")

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return status 301", func() {
			Expect(response.StatusCode).To(Equal(http.StatusMovedPermanently))

			url, err := response.Location()
			Expect(err).NotTo(HaveOccurred())
			Expect(url.Path).To(Equal("/auth/basic/token"))
		})
	})

	Context("GET /api/v1/user", func() {

		BeforeEach(func() {
			request, err = http.NewRequest("GET", server.URL+"/api/v1/user", nil)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Set("Content-Type", "application/json")

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return status 301", func() {
			Expect(response.StatusCode).To(Equal(http.StatusMovedPermanently))

			url, err := response.Location()
			Expect(err).NotTo(HaveOccurred())
			Expect(url.Path).To(Equal("/auth/userinfo"))
		})
	})
})
