package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/api/auth/authfakes"
	"github.com/concourse/concourse/atc/api/buildserver"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/skymarshal/token/tokenfakes"
)

var _ = Describe("WebAuthHandler", func() {
	var (
		fakeMiddleware *tokenfakes.FakeMiddleware
		fakeHandler    *authfakes.FakeHandler
	)

	var server *httptest.Server

	BeforeEach(func() {
		fakeMiddleware = new(tokenfakes.FakeMiddleware)
		fakeHandler = new(authfakes.FakeHandler)

		server = httptest.NewServer(auth.WebAuthHandler{
			Handler:    fakeHandler,
			Middleware: fakeMiddleware,
		})
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("handling a request", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			defer response.Body.Close()
		})

		Context("without the auth cookie", func() {
			BeforeEach(func() {
				fakeMiddleware.GetAuthTokenReturns("")
			})

			It("does not set auth cookie in response", func() {
				Expect(response.Cookies()).To(HaveLen(0))
			})

			It("proxies to the handler without setting the Authorization header", func() {
				Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
				_, r := fakeHandler.ServeHTTPArgsForCall(0)
				Expect(r.Header.Get("Authorization")).To(BeEmpty())
			})

			It("does not set CSRF required context in request", func() {
				Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
				_, r := fakeHandler.ServeHTTPArgsForCall(0)
				csrfRequiredContext := r.Context().Value(auth.CSRFRequiredKey)
				Expect(csrfRequiredContext).To(BeNil())
			})

			Context("the nested handler returns unauthorized", func() {
				BeforeEach(func() {
					fakeHandler.ServeHTTPStub = func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
					}
				})

				It("does not unset the auth cookie", func() {
					Expect(fakeMiddleware.UnsetAuthTokenCallCount()).To(Equal(0))
				})

				It("does not unset the csrf cookie", func() {
					Expect(fakeMiddleware.UnsetCSRFTokenCallCount()).To(Equal(0))
				})
			})
		})

		Context("with the auth cookie", func() {
			BeforeEach(func() {
				fakeMiddleware.GetAuthTokenReturns("username:password")
			})

			It("sets the Authorization header with the value from the cookie", func() {
				Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
				_, r := fakeHandler.ServeHTTPArgsForCall(0)
				Expect(r.Header.Get("Authorization")).To(Equal("username:password"))
			})

			It("sets CSRF required context in request", func() {
				Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
				_, r := fakeHandler.ServeHTTPArgsForCall(0)
				csrfRequiredContext := r.Context().Value(auth.CSRFRequiredKey)
				Expect(csrfRequiredContext).NotTo(BeNil())

				boolCsrf := csrfRequiredContext.(bool)
				Expect(boolCsrf).To(BeFalse())
			})

			Context("and the request also has an Authorization header", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "foobar")
				})

				It("does not override the Authorization header", func() {
					Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1))
					_, r := fakeHandler.ServeHTTPArgsForCall(0)
					Expect(r.Header.Get("Authorization")).To(Equal("foobar"))
				})
			})

			Context("the nested handler returns an event stream", func() {
				BeforeEach(func() {
					build := new(dbfakes.FakeBuild)
					fakeEventSource := new(dbfakes.FakeEventSource)
					fakeEventSource.NextReturns(event.Envelope{}, db.ErrEndOfBuildEventStream)
					build.EventsReturns(fakeEventSource, nil)

					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						defer GinkgoRecover()
						auth.WebAuthHandler{
							Handler:    buildserver.NewEventHandler(lager.NewLogger("test"), build),
							Middleware: fakeMiddleware,
						}.ServeHTTP(w, r)
					}))

					var err error
					request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns success", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("the nested handler returns unauthorized", func() {
				BeforeEach(func() {
					fakeHandler.ServeHTTPStub = func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
					}
				})

				It("unsets the auth cookie", func() {
					Expect(fakeMiddleware.UnsetAuthTokenCallCount()).To(Equal(1))
				})

				It("unsets the csrf cookie", func() {
					Expect(fakeMiddleware.UnsetCSRFTokenCallCount()).To(Equal(1))
				})
			})
		})
	})

	Describe("CSRF Required", func() {
		var request *http.Request
		var err error
		Context("when CSRF context is set", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
				Expect(err).To(BeNil())

				ctx := context.WithValue(request.Context(), auth.CSRFRequiredKey, true)
				request = request.WithContext(ctx)

			})
			It("fetches the bool value", func() {
				Expect(auth.IsCSRFRequired(request)).To(BeTrue())
			})
		})

		Context("when CSRF context is not set", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
				Expect(err).To(BeNil())
			})
			It("fetches the bool value", func() {
				Expect(auth.IsCSRFRequired(request)).To(BeFalse())
			})

		})
	})
})
