package baggageclaim_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/concourse/concourse/worker/baggageclaim/api"
	"github.com/concourse/concourse/worker/baggageclaim/client"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

var _ = Describe("Baggage Claim Client", func() {
	Describe("Interacting with the server", func() {
		var (
			bcServer *ghttp.Server
			bcClient baggageclaim.Client
		)

		BeforeEach(func() {
			bcServer = ghttp.NewServer()
			bcClient = client.New(bcServer.URL(), &http.Transport{DisableKeepAlives: true})
		})

		AfterEach(func() {
			bcServer.Close()
		})

		mockErrorResponse := func(method string, endpoint string, message string, status int) {
			response := fmt.Sprintf(`{"error":"%s"}`, message)
			bcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(method, endpoint),
					ghttp.RespondWith(status, response),
				),
			)
		}

		Describe("Looking up a volume by handle", func() {
			Context("when the volume does not exist", func() {
				It("reports that the volume could not be found", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes/some-handle"),
							ghttp.RespondWith(http.StatusNotFound, ""),
						),
					)
					foundVolume, found, err := bcClient.LookupVolume(context.Background(), "some-handle")
					Expect(foundVolume).To(BeNil())
					Expect(found).To(BeFalse())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("GET", "/volumes/some-handle", "lost baggage", http.StatusInternalServerError)
					foundVolume, found, err := bcClient.LookupVolume(context.Background(), "some-handle")
					Expect(foundVolume).To(BeNil())
					Expect(found).To(BeFalse())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Listing volumes", func() {
			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("GET", "/volumes", "lost baggage", http.StatusInternalServerError)
					volumes, err := bcClient.ListVolumes(context.Background(), baggageclaim.VolumeProperties{})
					Expect(volumes).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Destroying volumes", func() {
			Context("when all volumes are destroyed as requested", func() {
				var handles = []string{"some-handle"}
				var buf bytes.Buffer
				json.NewEncoder(&buf).Encode(handles)

				It("it returns all handles in response", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes/destroy"),
							ghttp.VerifyBody(buf.Bytes()),
							ghttp.RespondWithJSONEncoded(204, nil),
						))

					err := bcClient.DestroyVolumes(context.Background(), handles)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when no volumes are destroyed", func() {
				var handles = []string{"some-handle"}
				var buf bytes.Buffer
				json.NewEncoder(&buf).Encode(handles)

				It("it returns no handles", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes/destroy"),
							ghttp.VerifyBody(buf.Bytes()),
							ghttp.RespondWithJSONEncoded(500, handles),
						))

					err := bcClient.DestroyVolumes(context.Background(), handles)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("Destroying a single volume", func() {
			Context("when a volume is destroyed as requested", func() {
				var buf bytes.Buffer
				handle := "some-handle"
				_ = json.NewEncoder(&buf).Encode(handle)

				It("it does not return an error", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", fmt.Sprintf("/volumes/%s", handle)),
							ghttp.RespondWithJSONEncoded(204, nil),
						))

					err := bcClient.DestroyVolume(context.Background(), handle)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the volume could not be destroyed", func() {
				var buf bytes.Buffer
				handle := "some-handle"
				_ = json.NewEncoder(&buf).Encode(handle)

				It("it returns an error", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", fmt.Sprintf("/volumes/%s", handle)),
							ghttp.RespondWithJSONEncoded(500, handle),
						))

					err := bcClient.DestroyVolume(context.Background(), handle)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("Creating volumes", func() {

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/volumes-async"),
							ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
								Handle: "some-handle",
							}),
						),
					)
					mockErrorResponse("GET", "/volumes-async/some-handle", "lost baggage", http.StatusInternalServerError)
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
							ghttp.RespondWith(http.StatusNoContent, ""),
						),
					)

					createdVolume, err := bcClient.CreateVolume(context.Background(), "some-handle", baggageclaim.VolumeSpec{})
					Expect(createdVolume).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Stream in a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(context.Background(), "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("streams the volume", func() {
				bodyChan := make(chan []byte, 1)

				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/stream-in"),
						func(w http.ResponseWriter, r *http.Request) {
							str, _ := ioutil.ReadAll(r.Body)
							bodyChan <- str
						},
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
				err := vol.StreamIn(context.TODO(), ".", baggageclaim.GzipEncoding, strings.NewReader("some tar content"))
				Expect(err).ToNot(HaveOccurred())

				Expect(bodyChan).To(Receive(Equal([]byte("some tar content"))))
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-in", "lost baggage", http.StatusInternalServerError)
					err := vol.StreamIn(context.TODO(), "./some/path/", baggageclaim.GzipEncoding, strings.NewReader("even more tar"))
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Stream out a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(context.Background(), "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("streams the volume", func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/stream-out"),
						func(w http.ResponseWriter, r *http.Request) {
							w.Write([]byte("some tar content"))
						},
					),
				)
				out, err := vol.StreamOut(context.TODO(), ".", baggageclaim.GzipEncoding)
				Expect(err).NotTo(HaveOccurred())

				b, err := ioutil.ReadAll(out)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(b)).To(Equal("some tar content"))
			})

			Context("when error occurs", func() {
				It("returns API error message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", "lost baggage", http.StatusInternalServerError)
					_, err := vol.StreamOut(context.TODO(), "./some/path/", baggageclaim.GzipEncoding)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})

				It("returns ErrVolumeNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", "lost baggage", http.StatusNotFound)
					_, err := vol.StreamOut(context.TODO(), "./some/path/", baggageclaim.GzipEncoding)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
				})

				It("returns ErrFileNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", api.ErrStreamOutNotFound.Error(), http.StatusNotFound)
					_, err := vol.StreamOut(context.TODO(), "./some/path/", baggageclaim.GzipEncoding)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrFileNotFound))
				})
			})
		})

		Describe("Setting property on a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(context.Background(), "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when error occurs", func() {
				It("returns API error message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/properties/key", "lost baggage", http.StatusInternalServerError)
					err := vol.SetProperty(context.Background(), "key", "value")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})

				It("returns ErrVolumeNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/properties/key", "lost baggage", http.StatusNotFound)
					err := vol.SetProperty(context.Background(), "key", "value")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
				})
			})
		})

		Describe("Get p2p stream-in url", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(context.Background(), "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when api ok", func() {
				BeforeEach(func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/p2p-url"),
							ghttp.RespondWith(http.StatusOK, "http://some-url"),
						),
					)
				})
				It("should get the url", func() {
					url, err := vol.GetStreamInP2pUrl(context.TODO(), "some-path")
					Expect(err).ToNot(HaveOccurred())
					Expect(url).To(Equal("http://some-url/volumes/some-handle/stream-in?path=some-path"))
				})
			})

			Context("when error occurs", func() {
				BeforeEach(func() {
					mockErrorResponse("GET", "/p2p-url", "failed to get p2p url", http.StatusInternalServerError)
				})
				It("returns API error message", func() {
					url, err := vol.GetStreamInP2pUrl(context.TODO(), "some-path")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("failed to get p2p url: 500"))
					Expect(url).To(BeEmpty())
				})
			})
		})

		Describe("P2P stream out a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(context.Background(), "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when api succeeds", func() {
				BeforeEach(func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/volumes/some-handle/stream-p2p-out"),
							ghttp.RespondWith(http.StatusOK, ""),
						),
					)
				})
				It("should succeed", func() {
					err := vol.StreamP2pOut(context.TODO(), "some-dest-path", "http://some-url", "gzip")
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when error occurs", func() {
				BeforeEach(func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-p2p-out", "failed to p2p stream out", http.StatusInternalServerError)
				})
				It("returns API error message", func() {
					err := vol.StreamP2pOut(context.TODO(), "some-dest-path", "http://some-url", "gzip")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("failed to p2p stream out"))
				})
			})
		})
	})
})
