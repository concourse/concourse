package http_test

import (
	"net/http"

	"github.com/concourse/concourse/atc/creds"
	httpcreds "github.com/concourse/concourse/atc/creds/http"
	"github.com/concourse/concourse/vars"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("HTTP", func() {

	var (
		h         *httpcreds.HTTPSecretManager
		variables vars.Variables

		s *ghttp.Server
	)

	BeforeEach(func() {
		s = ghttp.NewServer()

		h = &httpcreds.HTTPSecretManager{
			URL: s.URL(),
		}

		variables = creds.NewVariables(h, "team", "pipeline", true)
	})

	AfterEach(func() {
		s.Close()
	})

	Describe("Get()", func() {
		It("should get secret from pipeline", func() {
			paths := []string{}

			handler := func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)

				if r.URL.Path == "/team/pipeline/foo" {
					w.Write([]byte("this secret value is a string"))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}

			s.AppendHandlers(handler)

			secret, found, err := variables.Get(
				vars.Reference{Path: "foo"},
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(Equal(true))
			Expect(secret).To(Equal("this secret value is a string"))
			Expect(paths).To(ContainElement("/team/pipeline/foo"))
		})

		It("should get secret from team", func() {
			paths := []string{}

			handler := func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)

				if r.URL.Path == "/team/foo" {
					w.Write([]byte("this secret value is a string"))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}

			s.AppendHandlers(handler, handler)

			secret, found, err := variables.Get(
				vars.Reference{Path: "foo"},
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(Equal(true))
			Expect(secret).To(Equal("this secret value is a string"))
			Expect(paths).To(ContainElement("/team/foo"))
		})

		It("should get secret from root", func() {
			paths := []string{}

			handler := func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)

				if r.URL.Path == "/foo" {
					w.Write([]byte("this secret value is a string"))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}

			s.AppendHandlers(handler, handler, handler)

			secret, found, err := variables.Get(
				vars.Reference{Path: "foo"},
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(Equal(true))
			Expect(secret).To(Equal("this secret value is a string"))
			Expect(paths).To(ContainElement("/foo"))
		})

		Context("when not allowing the root path", func() {
			BeforeEach(func() {
				variables = creds.NewVariables(h, "team", "pipeline", false)
			})

			It("should not request the root", func() {
				paths := []string{}

				handler := func(w http.ResponseWriter, r *http.Request) {
					paths = append(paths, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}

				s.AppendHandlers(handler, handler, handler)

				secret, found, err := variables.Get(
					vars.Reference{Path: "foo"},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(Equal(false))
				Expect(secret).To(BeNil())
				Expect(paths).NotTo(ContainElement("/foo"))
			})
		})

		Context("when returning a YAML secret", func() {
			It("should deserialize", func() {
				handler := func(w http.ResponseWriter, r *http.Request) {
					w.Header().Add("Content-Type", "text/yaml")
					w.Write([]byte("---\n'this is a': 'yaml secret'"))
				}

				s.AppendHandlers(handler)

				secret, found, err := variables.Get(
					vars.Reference{Path: "foo"},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(Equal(true))
				Expect(secret).To(Equal(map[string]interface{}{
					"this is a": "yaml secret",
				}))
			})
		})

		Context("when returning a JSON secret", func() {
			It("should deserialize", func() {
				handler := func(w http.ResponseWriter, r *http.Request) {
					w.Header().Add("Content-Type", "application/json")
					w.Write([]byte(`{"this is a": "json secret"}`))
				}

				s.AppendHandlers(handler)

				secret, found, err := variables.Get(
					vars.Reference{Path: "foo"},
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(Equal(true))
				Expect(secret).To(Equal(map[string]interface{}{
					"this is a": "json secret",
				}))
			})
		})
	})
})
