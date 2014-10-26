package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"sync"

	tbuilds "github.com/concourse/turbine/api/builds"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc/builds"
)

var _ = Describe("Builds API", func() {
	Describe("POST /api/v1/builds", func() {
		var turbineBuild tbuilds.Build

		var response *http.Response

		BeforeEach(func() {
			turbineBuild = tbuilds.Build{
				Config: tbuilds.Config{
					Run: tbuilds.RunConfig{
						Path: "ls",
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(turbineBuild)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds", bytes.NewBuffer(reqPayload))
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when creating a one-off build succeeds", func() {
			BeforeEach(func() {
				buildsDB.CreateOneOffBuildReturns(builds.Build{
					ID:      42,
					Name:    "1",
					JobName: "job1",
					Status:  builds.StatusStarted,
				}, nil)
			})

			Context("and building succeeds", func() {
				It("returns 201 Created", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusCreated))
				})

				It("returns the build", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`{"id": 42, "name": "1", "job_name": "job1", "status": "started"}`))
				})

				It("executes a one-off build", func() {
					Ω(buildsDB.CreateOneOffBuildCallCount()).Should(Equal(1))

					Ω(builder.BuildCallCount()).Should(Equal(1))
					oneOff, tBuild := builder.BuildArgsForCall(0)
					Ω(oneOff).Should(Equal(builds.Build{
						ID:      42,
						Name:    "1",
						JobName: "job1",
						Status:  builds.StatusStarted,
					}))
					Ω(tBuild).Should(Equal(turbineBuild))
				})
			})

			Context("and building fails", func() {
				BeforeEach(func() {
					builder.BuildReturns(errors.New("oh no!"))
				})

				It("returns 500 Internal Server Error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when creating a one-off build fails", func() {
			BeforeEach(func() {
				buildsDB.CreateOneOffBuildReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/builds", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/builds")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting all builds succeeds", func() {
			BeforeEach(func() {
				buildsDB.GetAllBuildsReturns([]builds.Build{
					{
						ID:      3,
						Name:    "2",
						JobName: "job2",
						Status:  builds.StatusStarted,
					},
					{
						ID:      1,
						Name:    "1",
						JobName: "job1",
						Status:  builds.StatusSucceeded,
					},
				}, nil)
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns all builds", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`[
					{"id": 3, "name": "2", "job_name": "job2", "status": "started"},
					{"id": 1, "name": "1", "job_name": "job1", "status": "succeeded"}
				]`))
			})
		})

		Context("when getting all builds fails", func() {
			BeforeEach(func() {
				buildsDB.GetAllBuildsReturns(nil, errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/builds/:build_id/events", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("GET", server.URL+"/api/v1/builds/128/events", nil)
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("serves the request via the event handler with no censor", func() {
			Ω(response.StatusCode).Should(Equal(200))

			body, err := ioutil.ReadAll(response.Body)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(string(body)).Should(Equal("fake event handler factory was here"))

			Ω(constructedEventHandler.db).Should(Equal(buildsDB))
			Ω(constructedEventHandler.buildID).Should(Equal(128))
			Ω(constructedEventHandler.censor).Should(BeNil())
		})
	})

	Describe("POST /api/v1/builds/:build_id/abort", func() {
		var (
			abortTarget *ghttp.Server

			response *http.Response
		)

		BeforeEach(func() {
			abortTarget = ghttp.NewServer()

			abortTarget.AppendHandlers(
				ghttp.VerifyRequest("POST", "/builds/some-guid/abort"),
			)

			buildsDB.GetBuildReturns(builds.Build{
				ID:       128,
				Guid:     "some-guid",
				Endpoint: abortTarget.URL(),
			}, nil)
		})

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("POST", server.URL+"/api/v1/builds/128/abort", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			abortTarget.Close()
		})

		Context("when the build can be aborted", func() {
			BeforeEach(func() {
				buildsDB.SaveBuildStatusReturns(nil)
			})

			It("aborts the build via its abort callback", func() {
				Ω(abortTarget.ReceivedRequests()).Should(HaveLen(1))
			})

			Context("and the abort callback returns a status code", func() {
				BeforeEach(func() {
					abortTarget.SetHandler(0, func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusTeapot)
					})
				})

				It("forwards it", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusTeapot))
				})
			})

			Context("and the abort callback fails", func() {
				BeforeEach(func() {
					abortTarget.SetHandler(0, func(w http.ResponseWriter, r *http.Request) {
						abortTarget.HTTPTestServer.CloseClientConnections()
					})
				})

				It("returns 500 Internal Server Error", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when the build cannot be aborted", func() {
			BeforeEach(func() {
				buildsDB.SaveBuildStatusReturns(errors.New("oh no!"))
			})

			It("returns 500 Internal Server Error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("POST /api/v1/builds/:build_id/hijack", func() {
		var (
			hijackTarget *ghttp.Server

			response *http.Response

			buildHijackConns   <-chan net.Conn
			buildHijackReaders <-chan *gbytes.Buffer

			clientConn   net.Conn
			clientReader io.Reader
		)

		BeforeEach(func() {
			hijackedConns := make(chan net.Conn, 1)
			buildHijackConns = hijackedConns

			hijackedReaders := make(chan *gbytes.Buffer, 1)
			buildHijackReaders = hijackedReaders

			hijackTarget = ghttp.NewServer()
			hijackTarget.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/builds/some-guid/hijack"),
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)

					var msg json.RawMessage
					err := json.NewDecoder(r.Body).Decode(&msg)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(string(msg)).Should(Equal(string(`{"some":"hijack-body"}`)))

					conn, br, err := w.(http.Hijacker).Hijack()
					Ω(err).ShouldNot(HaveOccurred())

					defer conn.Close()

					buf := gbytes.NewBuffer()

					hijackedConns <- conn
					hijackedReaders <- buf

					io.Copy(buf, br)
				},
			))
		})

		JustBeforeEach(func() {
			var err error

			hijackReq, err := http.NewRequest(
				"POST",
				server.URL+"/api/v1/builds/128/hijack",
				bytes.NewBufferString(`{"some":"hijack-body"}`),
			)
			Ω(err).ShouldNot(HaveOccurred())

			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			Ω(err).ShouldNot(HaveOccurred())

			client := httputil.NewClientConn(conn, nil)

			response, err = client.Do(hijackReq)
			Ω(err).ShouldNot(HaveOccurred())

			clientConn, clientReader = client.Hijack()
		})

		AfterEach(func() {
			clientConn.Close()
			hijackTarget.Close()
		})

		Context("when the build can be found", func() {
			Context("and it has a hijack URL", func() {
				BeforeEach(func() {
					buildsDB.GetBuildReturns(builds.Build{
						ID:       128,
						Guid:     "some-guid",
						Endpoint: hijackTarget.URL(),
					}, nil)
				})

				It("proxies all traffic via a hijacked connection", func() {
					var serverReceivedBuf *gbytes.Buffer
					Eventually(buildHijackReaders).Should(Receive(&serverReceivedBuf))

					var serverConnectedConn net.Conn
					Eventually(buildHijackConns).Should(Receive(&serverConnectedConn))

					clientReceivedBuf := gbytes.NewBuffer()

					readingFromServer := new(sync.WaitGroup)
					readingFromServer.Add(1)
					go func() {
						io.Copy(clientReceivedBuf, clientReader)
						readingFromServer.Done()
					}()

					_, err := clientConn.Write([]byte("hello from client"))
					Ω(err).ShouldNot(HaveOccurred())

					Eventually(serverReceivedBuf).Should(gbytes.Say("hello from client"))

					_, err = serverConnectedConn.Write([]byte("hello from server"))
					Ω(err).ShouldNot(HaveOccurred())

					err = serverConnectedConn.Close()
					Ω(err).ShouldNot(HaveOccurred())

					readingFromServer.Wait()

					Eventually(clientReceivedBuf).Should(gbytes.Say("hello from server"))
				})
			})

			Context("but it does not have a hijack URL", func() {
				BeforeEach(func() {
					buildsDB.GetBuildReturns(builds.Build{ID: 128}, nil)
				})

				It("returns 400 Bad Request", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("when the build cannot be found", func() {
			BeforeEach(func() {
				buildsDB.GetBuildReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})
})
