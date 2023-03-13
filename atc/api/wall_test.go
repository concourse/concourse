package api_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/testhelpers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Wall API", func() {
	var response *http.Response
	Context("Gets a wall message", func() {
		BeforeEach(func() {
			dbWall.GetWallReturns(atc.Wall{Message: "test message"}, nil)
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
			expectedHeaderEntries := map[string]string{
				"Content-Type": "application/json",
			}
			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
		})

		Context("the message does not expire", func() {

			It("returns only message", func() {
				Expect(dbWall.GetWallCallCount()).To(Equal(1))
				Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"message":"test message"}`))
			})
		})

		Context("and the message does expire", func() {
			var (
				expectedDuration time.Duration
			)
			BeforeEach(func() {
				expiresAt := time.Now().Add(time.Minute)
				expectedDuration = time.Until(expiresAt)
				dbWall.GetWallReturns(atc.Wall{Message: "test message", TTL: expectedDuration}, nil)
			})

			It("returns the expiration with the message", func() {
				Expect(dbWall.GetWallCallCount()).To(Equal(1))

				var msg atc.Wall
				err := json.NewDecoder(response.Body).Decode(&msg)
				Expect(err).ToNot(HaveOccurred())
				Expect(msg).To(Equal(atc.Wall{
					Message: "test message",
					TTL:     expectedDuration,
				}))
			})
		})
	})

	Context("Sets a wall message", func() {
		var expectedWall atc.Wall
		BeforeEach(func() {
			expectedWall = atc.Wall{
				Message: "test message",
				TTL:     time.Minute,
			}

			dbWall.SetWallReturns(nil)
		})

		JustBeforeEach(func() {
			payload, err := json.Marshal(expectedWall)
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
					Expect(dbWall.SetWallCallCount()).To(Equal(1))
					Expect(dbWall.SetWallArgsForCall(0)).To(Equal(expectedWall))
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
})
