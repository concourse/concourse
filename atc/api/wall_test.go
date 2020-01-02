package api_test

import (
	"bytes"
	"encoding/json"
	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
	"time"
)

var _ = Describe("Wall API", func() {
	var response *http.Response
	Context("Gets a wall message", func() {
		BeforeEach(func() {
			dbWall.GetMessageReturns("test message", nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/wall", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns Content-Type 'application/json'", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		Context("the message does not expire", func() {

			It("returns only message", func() {
				Expect(dbWall.GetMessageCallCount()).To(Equal(1))

				Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"message":"test message"}`))
			})
		})

		Context("and the message does expire", func() {
			var (
				expectedDuration string
			)
			BeforeEach(func() {
				expiresAt := time.Now().Add(time.Minute)
				expectedDuration = time.Until(expiresAt).Round(time.Second).String()
				dbWall.GetExpirationReturns(expiresAt, nil)
			})

			It("returns the expiration with the message", func() {
				Expect(dbWall.GetMessageCallCount()).To(Equal(1))
				Expect(dbWall.GetExpirationCallCount()).To(Equal(1))

				var msg atc.Wall
				json.NewDecoder(response.Body).Decode(&msg)
				Expect(msg).To(Equal(atc.Wall{
					Message:   "test message",
					ExpiresIn: expectedDuration,
				}))
			})
		})
	})

	Context("Sets a wall message", func() {
		var message atc.Wall
		BeforeEach(func() {
			message = atc.Wall{
				Message:   "test message",
				ExpiresIn: "1m",
			}

			dbWall.SetMessageReturns(nil)
			dbWall.SetExpirationReturns(nil)
		})

		JustBeforeEach(func() {
			payload, err := json.Marshal(message)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/api/v1/wall",
				ioutil.NopCloser(bytes.NewBuffer(payload)))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("and is admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("sets the message and expiration", func() {
					Expect(dbWall.SetMessageCallCount()).To(Equal(1), "message")
					Expect(dbWall.SetExpirationCallCount()).To(Equal(1), "expires_in")

					Expect(dbWall.SetMessageArgsForCall(0)).To(Equal("test message"))
					Expect(dbWall.SetExpirationArgsForCall(0)).To(Equal(time.Minute))
				})
			})
			Context("and is not admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})
		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Context("Clears the wall message", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("DELETE", server.URL+"/api/v1/wall", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("is an admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("makes the Clear database call", func() {
					Expect(dbWall.ClearCallCount()).To(Equal(1))
				})
			})
			Context("is not an admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})
		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

		})

	})

	Context("Gets the expiration", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/wall/expiration", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns Content-Type 'application/json'", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		Context("when a message is set", func() {
			Context("and has an expiration", func() {
				var expectedTime time.Time

				BeforeEach(func() {
					expectedTime = time.Now().Add(time.Minute)
					dbWall.GetExpirationReturns(expectedTime, nil)
				})

				It("returns only the expiration", func() {
					Expect(dbWall.GetExpirationCallCount()).To(Equal(1))

					var expires atc.Wall
					json.NewDecoder(response.Body).Decode(&expires)
					Expect(expires).To(Equal(atc.Wall{
						ExpiresIn: time.Until(expectedTime).Round(time.Second).String(),
					}))
				})
			})

			Context("and has no expiration", func() {
				BeforeEach(func() {
					dbWall.GetExpirationReturns(time.Time{}, nil)
				})

				It("an empty json block", func() {
					Expect(dbWall.GetExpirationCallCount()).To(Equal(1))
					Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{}`))
				})
			})
		})

		Context("when a message is not set", func() {
			BeforeEach(func() {
				dbWall.GetExpirationReturns(time.Time{}, nil)
			})

			It("an empty json block", func() {
				Expect(dbWall.GetExpirationCallCount()).To(Equal(1))
				Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{}`))
			})
		})
	})

	Context("Sets the expiration", func() {
		var message atc.Wall
		BeforeEach(func() {
			message = atc.Wall{
				ExpiresIn: "1m",
			}

			dbWall.SetExpirationReturns(nil)
		})

		JustBeforeEach(func() {
			payload, err := json.Marshal(message)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest("PUT", server.URL+"/api/v1/wall/expiration",
				ioutil.NopCloser(bytes.NewBuffer(payload)))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			It("returns 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})

			Context("and are an admin", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("updates the expiration", func() {
					Expect(dbWall.SetExpirationCallCount()).To(Equal(1))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
