package auth_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/github/fakes"
)

var _ = Describe("GitHubAuthHandler", func() {
	var server *httptest.Server
	var client *http.Client
	var fakeGitHubClient *fakes.FakeClient

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffer := bytes.NewBufferString("github ")

		io.Copy(w, buffer)
		io.Copy(w, r.Body)
	})

	BeforeEach(func() {
		fakeGitHubClient = new(fakes.FakeClient)
		authHandler := auth.Handler{
			Handler: simpleHandler,
			Validator: auth.GitHubOrganizationValidator{
				Organization: "testOrg",
				Client:       fakeGitHubClient,
			},
		}

		server = httptest.NewServer(authHandler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	AfterEach(func() {
		server.Close()
	})

	It("strips out the Token keyword before calling to github", func() {
		requestBody := bytes.NewBufferString("hello")
		request, err := http.NewRequest("GET", server.URL, requestBody)
		Expect(err).NotTo(HaveOccurred())
		request.Header.Add("Authorization", "Token abcd")

		_, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeGitHubClient.GetOrganizationsCallCount()).To(Equal(1))

		Expect(fakeGitHubClient.GetOrganizationsArgsForCall(0)).To(Equal("abcd"))
	})

	It("returns a 401 when the Token is not passed in the header properly", func() {
		requestBody := bytes.NewBufferString("hello")
		request, err := http.NewRequest("GET", server.URL, requestBody)
		Expect(err).NotTo(HaveOccurred())

		response, err := client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))

		request, err = http.NewRequest("GET", server.URL, requestBody)
		request.Header.Add("Authorization", "abcd")

		response, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))

		Expect(fakeGitHubClient.GetOrganizationsCallCount()).To(Equal(0))
	})

	Context("when authentication fails", func() {
		Context("because the github client errors", func() {
			BeforeEach(func() {
				fakeGitHubClient.GetOrganizationsReturns(nil, errors.New("disaster"))
			})

			It("returns 401", func() {
				requestBody := bytes.NewBufferString("hello")
				request, err := http.NewRequest("GET", server.URL, requestBody)
				Expect(err).NotTo(HaveOccurred())
				request.Header.Add("Authorization", "Token besttoken")

				response, err := client.Do(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))

				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(Equal("not authorized"))

				Expect(fakeGitHubClient.GetOrganizationsCallCount()).To(Equal(1))
			})
		})

		Context("because the given token is not in the org", func() {
			BeforeEach(func() {
				fakeGitHubClient.GetOrganizationsReturns([]string{"nope"}, nil)
			})

			It("returns 401", func() {
				requestBody := bytes.NewBufferString("hello")
				request, err := http.NewRequest("GET", server.URL, requestBody)
				request.Header.Add("Authorization", "Token besttoken")
				Expect(err).NotTo(HaveOccurred())

				response, err := client.Do(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))

				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(Equal("not authorized"))

				Expect(fakeGitHubClient.GetOrganizationsCallCount()).To(Equal(1))
			})
		})

	})

	Context("with the correct credentials", func() {
		BeforeEach(func() {
			fakeGitHubClient.GetOrganizationsReturns([]string{"testOrg"}, nil)
		})

		It("It athenticates with GitHub", func() {
			requestBody := bytes.NewBufferString("hello")
			request, err := http.NewRequest("GET", server.URL, requestBody)
			Expect(err).NotTo(HaveOccurred())
			request.Header.Add("Authorization", "Token abcd")

			response, err := client.Do(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			responseBody, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(responseBody)).To(Equal("github hello"))
		})
	})
})
