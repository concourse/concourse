package token_test

import (
	"time"

	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"

	"github.com/concourse/concourse/skymarshal/token"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Token Middleware", func() {
	It("GetToken is the inverse of SetToken when cookies are supported", func() {
		tokenString := "some-type some-token"
		middleware := token.NewMiddleware(false)

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			switch req.URL.Path {
			case "/set":
				middleware.SetToken(w, tokenString, time.Now().Add(1*time.Hour))
			case "/get":
				defer GinkgoRecover()
				Expect(middleware.GetToken(req)).To(Equal(tokenString))
			}
		}))
		client := testServer.Client()
		cookieJar, err := cookiejar.New(nil)
		Expect(err).ToNot(HaveOccurred())
		client.Jar = cookieJar

		request, err := http.NewRequest("GET", testServer.URL+"/set", nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		request, err = http.NewRequest("GET", testServer.URL+"/get", nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		testServer.Close()
	})
})
