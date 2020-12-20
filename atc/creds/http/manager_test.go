package http_test

import (
	"net/http"

	httpcreds "github.com/concourse/concourse/atc/creds/http"
	flags "github.com/jessevdk/go-flags"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("HTTPManager", func() {
	var (
		manager httpcreds.HTTPManager

		s *ghttp.Server
	)

	BeforeEach(func() {
		s = ghttp.NewServer()
	})

	AfterEach(func() {
		s.Close()
	})

	Describe("IsConfigured()", func() {
		BeforeEach(func() {
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
		})

		It("fails on empty HTTPManager", func() {
			Expect(manager.IsConfigured()).To(BeFalse())
		})

		It("passes if URL is set", func() {
			manager.URL = "hello://world"
			Expect(manager.IsConfigured()).To(BeTrue())
		})
	})

	Describe("Health()", func() {
		BeforeEach(func() {
			manager.URL = s.URL()
		})

		Context("when the server is healthy", func() {
			BeforeEach(func() {
				s.RouteToHandler("GET", "/health", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
			})

			It("does not error", func() {
				_, err := manager.Health()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the server is not healthy", func() {
			BeforeEach(func() {
				s.RouteToHandler("GET", "/health", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				})
			})

			It("errors", func() {
				_, err := manager.Health()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
