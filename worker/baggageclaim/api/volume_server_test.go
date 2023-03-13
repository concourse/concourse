package api_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/concourse/go-archive/tarfs"
	"github.com/klauspost/compress/zstd"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/go-archive/tgzfs"

	"github.com/concourse/concourse/worker/baggageclaim"
	"github.com/concourse/concourse/worker/baggageclaim/api"
	"github.com/concourse/concourse/worker/baggageclaim/uidgid"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"
)

var _ = Describe("Volume Server", func() {
	var (
		handler http.Handler

		volumeDir string
		tempDir   string
	)

	BeforeEach(func() {
		var err error

		tempDir, err = ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelProcess()))
		Expect(err).NotTo(HaveOccurred())

		// ioutil.TempDir creates it 0700; we need public readability for
		// unprivileged StreamIn
		err = os.Chmod(tempDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		volumeDir = tempDir
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("volume-server")

		fs, err := volume.NewFilesystem(&driver.NaiveDriver{}, volumeDir)
		Expect(err).NotTo(HaveOccurred())

		var privilegedNamespacer, unprivilegedNamespacer uidgid.Namespacer
		if runtime.GOOS == "linux" {
			privilegedNamespacer = &uidgid.UidNamespacer{
				Translator: uidgid.NewTranslator(uidgid.NewPrivilegedMapper()),
				Logger:     logger.Session("uid-namespacer"),
			}

			unprivilegedNamespacer = &uidgid.UidNamespacer{
				Translator: uidgid.NewTranslator(uidgid.NewUnprivilegedMapper()),
				Logger:     logger.Session("uid-namespacer"),
			}
		} else {
			privilegedNamespacer = &uidgid.NoopNamespacer{}
			unprivilegedNamespacer = &uidgid.NoopNamespacer{}
		}

		repo := volume.NewRepository(
			fs,
			volume.NewLockManager(),
			privilegedNamespacer,
			unprivilegedNamespacer,
		)

		strategerizer := volume.NewStrategerizer()

		re := regexp.MustCompile("lo")
		handler, err = api.NewHandler(logger, strategerizer, repo, re, 4, 7766)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// keep trying; async volume creation tests can cause failures on windows
		// while multiple processes have a file open
		Eventually(func() error {
			return os.RemoveAll(tempDir)
		}, time.Minute).ShouldNot(HaveOccurred())
	})

	Describe("listing the volumes", func() {
		var recorder *httptest.ResponseRecorder

		JustBeforeEach(func() {
			recorder = httptest.NewRecorder()
			request, _ := http.NewRequest("GET", "/volumes", nil)

			handler.ServeHTTP(recorder, request)
		})

		Context("when there are no volumes", func() {
			It("returns an empty array", func() {
				Expect(recorder.Body).To(MatchJSON(`[]`))
			})
		})
	})

	Describe("querying for volumes with properties", func() {
		props := baggageclaim.VolumeProperties{
			"property-query": "value",
		}

		It("finds volumes that have a property", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				Properties: props,
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			body.Reset()
			err = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "another-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
			})
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-query=value", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())

			Expect(volumes).To(HaveLen(1))
		})

		It("returns an error if an invalid set of properties are specified", func() {
			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("GET", "/volumes?property-query=value&property-query=another-value", nil)
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(422))
		})
	})

	Describe("streaming tar files into volumes", func() {
		var (
			myVolume     volume.Volume
			tgzBuffer    *bytes.Buffer
			isPrivileged bool
		)

		JustBeforeEach(func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				Privileged: isPrivileged,
			})
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", "/volumes", body)
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			err = json.NewDecoder(recorder.Body).Decode(&myVolume)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when tar file is valid", func() {

			Context("when using gzip encoding", func() {
				BeforeEach(func() {
					tgzBuffer = new(bytes.Buffer)
					gzWriter := gzip.NewWriter(tgzBuffer)
					defer gzWriter.Close()

					tarWriter := tar.NewWriter(gzWriter)
					defer tarWriter.Close()

					err := tarWriter.WriteHeader(&tar.Header{
						Name: "some-file",
						Mode: 0600,
						Size: int64(len("file-content")),
					})
					Expect(err).NotTo(HaveOccurred())
					_, err = tarWriter.Write([]byte("file-content"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("extracts the tar stream into the volume's DataPath", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tgzBuffer)
					request.Header.Set("Content-Encoding", string(baggageclaim.GzipEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(204))

					tarContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
					Expect(tarContentsPath).To(BeAnExistingFile())

					Expect(ioutil.ReadFile(tarContentsPath)).To(Equal([]byte("file-content")))
				})

			})

			Context("when using zstd encoding", func() {
				BeforeEach(func() {
					tgzBuffer = new(bytes.Buffer)
					zstdWriter, err := zstd.NewWriter(tgzBuffer)
					Expect(err).ToNot(HaveOccurred())
					defer zstdWriter.Close()

					tarWriter := tar.NewWriter(zstdWriter)
					defer tarWriter.Close()

					err = tarWriter.WriteHeader(&tar.Header{
						Name: "some-file",
						Mode: 0600,
						Size: int64(len("file-content")),
					})
					Expect(err).NotTo(HaveOccurred())
					_, err = tarWriter.Write([]byte("file-content"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("extracts the tar stream into the volume's DataPath", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tgzBuffer)
					request.Header.Set("Content-Encoding", string(baggageclaim.ZstdEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(204))

					tarContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
					Expect(tarContentsPath).To(BeAnExistingFile())

					Expect(ioutil.ReadFile(tarContentsPath)).To(Equal([]byte("file-content")))
				})
			})
		})

		Context("when the tar stream is invalid", func() {
			BeforeEach(func() {
				tgzBuffer = new(bytes.Buffer)
				tgzBuffer.Write([]byte("This is an invalid tar file!"))
			})

			Context("when using gzip encoding", func() {
				It("returns 400 when err is exitError", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in", myVolume.Handle), tgzBuffer)
					request.Header.Set("Content-Encoding", string(baggageclaim.GzipEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(400))
				})

			})
			Context("when using zstd encoding", func() {
				It("returns 400 when err is exitError", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in", myVolume.Handle), tgzBuffer)
					request.Header.Set("Content-Encoding", string(baggageclaim.ZstdEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(400))
				})
			})
		})

		Context("when using an unsupported encoding", func() {
			BeforeEach(func() {
				tgzBuffer = new(bytes.Buffer)
			})

			It("returns 400 when err is UnsupportedEncodingError", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in", myVolume.Handle), tgzBuffer)
				request.Header.Set("Content-Encoding", "unsupported")
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(400))
			})
		})

		It("returns 404 when volume is not found", func() {
			tgzBuffer = new(bytes.Buffer)
			request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in", "invalid-handle"), tgzBuffer)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(404))
		})
	})

	Describe("streaming tar out of a volume", func() {
		var (
			myVolume  volume.Volume
			tarBuffer *bytes.Buffer
			encoding  string
		)

		BeforeEach(func() {
			tarBuffer = new(bytes.Buffer)
		})

		JustBeforeEach(func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
			})
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", "/volumes", body)
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			err = json.NewDecoder(recorder.Body).Decode(&myVolume)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 404 when source path is invalid", func() {
			request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "bogus-path"), nil)
			request.Header.Set("Accept-Encoding", string(baggageclaim.GzipEncoding))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(404))

			var responseError *api.ErrorResponse
			err := json.NewDecoder(recorder.Body).Decode(&responseError)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseError.Message).To(Equal("no such file or directory"))
		})

		Context("when streaming a file", func() {

			JustBeforeEach(func() {
				streamInRequest, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tarBuffer)
				streamInRequest.Header.Set("Content-Encoding", encoding)
				streamInRecorder := httptest.NewRecorder()
				handler.ServeHTTP(streamInRecorder, streamInRequest)
				Expect(streamInRecorder.Code).To(Equal(204))

				tarContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
				Expect(tarContentsPath).To(BeAnExistingFile())

				Expect(ioutil.ReadFile(tarContentsPath)).To(Equal([]byte("file-content")))
			})

			Context("when using gzip encoding", func() {
				BeforeEach(func() {
					gzWriter := gzip.NewWriter(tarBuffer)
					defer gzWriter.Close()

					tarWriter := tar.NewWriter(gzWriter)
					defer tarWriter.Close()

					err := tarWriter.WriteHeader(&tar.Header{
						Name: "some-file",
						Mode: 0600,
						Size: int64(len("file-content")),
					})
					Expect(err).NotTo(HaveOccurred())
					_, err = tarWriter.Write([]byte("file-content"))
					Expect(err).NotTo(HaveOccurred())
					encoding = string(baggageclaim.GzipEncoding)
				})

				It("creates a tar", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "dest-path"), nil)
					request.Header.Set("Accept-Encoding", string(baggageclaim.GzipEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(200))

					unpackedDir := filepath.Join(tempDir, "unpacked-dir")
					err := os.MkdirAll(unpackedDir, os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
					defer os.RemoveAll(unpackedDir)

					err = tgzfs.Extract(recorder.Body, unpackedDir)
					Expect(err).NotTo(HaveOccurred())

					fileInfo, err := os.Stat(filepath.Join(unpackedDir, "some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeFalse())
					Expect(fileInfo.Size()).To(Equal(int64(len("file-content"))))

					contents, err := ioutil.ReadFile(filepath.Join(unpackedDir, "./some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("file-content"))
				})
			})

			Context("when using zstd encoding", func() {
				BeforeEach(func() {
					zstdWriter, err := zstd.NewWriter(tarBuffer)
					Expect(err).NotTo(HaveOccurred())
					defer zstdWriter.Close()

					tarWriter := tar.NewWriter(zstdWriter)
					defer tarWriter.Close()

					err = tarWriter.WriteHeader(&tar.Header{
						Name: "some-file",
						Mode: 0600,
						Size: int64(len("file-content")),
					})
					Expect(err).NotTo(HaveOccurred())
					_, err = tarWriter.Write([]byte("file-content"))
					Expect(err).NotTo(HaveOccurred())
					encoding = string(baggageclaim.ZstdEncoding)
				})

				It("creates a tar", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "dest-path"), nil)
					request.Header.Set("Accept-Encoding", string(baggageclaim.ZstdEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(200))

					unpackedDir := filepath.Join(tempDir, "unpacked-dir")
					err := os.MkdirAll(unpackedDir, os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
					defer os.RemoveAll(unpackedDir)

					zstdReader, err := zstd.NewReader(recorder.Body)
					Expect(err).NotTo(HaveOccurred())
					defer zstdReader.Close()
					err = tarfs.Extract(zstdReader, unpackedDir)
					Expect(err).NotTo(HaveOccurred())

					fileInfo, err := os.Stat(filepath.Join(unpackedDir, "some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeFalse())
					Expect(fileInfo.Size()).To(Equal(int64(len("file-content"))))

					contents, err := ioutil.ReadFile(filepath.Join(unpackedDir, "./some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("file-content"))
				})
			})
		})

		Context("when streaming a directory", func() {
			var tarDir string

			BeforeEach(func() {
				tarDir = filepath.Join(tempDir, "tar-dir")

				err := os.MkdirAll(filepath.Join(tarDir, "sub"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(tarDir, "sub", "some-file"), []byte("some-file-content"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(tarDir, "other-file"), []byte("other-file-content"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				tarBuffer = new(bytes.Buffer)
			})

			JustBeforeEach(func() {
				streamInRequest, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tarBuffer)
				streamInRequest.Header.Set("Content-Encoding", encoding)
				streamInRecorder := httptest.NewRecorder()
				handler.ServeHTTP(streamInRecorder, streamInRequest)
				Expect(streamInRecorder.Code).To(Equal(204))

				tarContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path")
				Expect(tarContentsPath).To(BeADirectory())
			})

			Context("when using gzip encoding", func() {
				BeforeEach(func() {
					err := tgzfs.Compress(tarBuffer, tarDir, ".")
					Expect(err).NotTo(HaveOccurred())
					encoding = string(baggageclaim.GzipEncoding)
				})

				It("creates a tar", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "dest-path"), nil)
					request.Header.Set("Accept-Encoding", string(baggageclaim.GzipEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(200))

					unpackedDir := filepath.Join(tempDir, "unpacked-dir")
					err := os.MkdirAll(unpackedDir, os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
					defer os.RemoveAll(unpackedDir)

					err = tgzfs.Extract(recorder.Body, unpackedDir)
					Expect(err).NotTo(HaveOccurred())

					fileInfo, err := os.Stat(filepath.Join(unpackedDir, "other-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeFalse())
					Expect(fileInfo.Size()).To(Equal(int64(len("other-file-content"))))

					contents, err := ioutil.ReadFile(filepath.Join(unpackedDir, "other-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("other-file-content"))

					dirInfo, err := os.Stat(filepath.Join(unpackedDir, "sub"))
					Expect(err).NotTo(HaveOccurred())
					Expect(dirInfo.IsDir()).To(BeTrue())

					fileInfo, err = os.Stat(filepath.Join(unpackedDir, "sub/some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeFalse())
					Expect(fileInfo.Size()).To(Equal(int64(len("some-file-content"))))

					contents, err = ioutil.ReadFile(filepath.Join(unpackedDir, "sub/some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("some-file-content"))
				})
			})

			Context("when using zstd encoding", func() {
				BeforeEach(func() {
					zstdWriter, err := zstd.NewWriter(tarBuffer)
					Expect(err).ToNot(HaveOccurred())
					err = tarfs.Compress(zstdWriter, tarDir, ".")
					Expect(err).NotTo(HaveOccurred())
					zstdWriter.Close()
					encoding = string(baggageclaim.ZstdEncoding)
				})

				It("creates a tar", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "dest-path"), nil)
					request.Header.Set("Accept-Encoding", string(baggageclaim.ZstdEncoding))
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(200))

					unpackedDir := filepath.Join(tempDir, "unpacked-dir")
					err := os.MkdirAll(unpackedDir, os.ModePerm)
					Expect(err).NotTo(HaveOccurred())
					defer os.RemoveAll(unpackedDir)

					tarByteReader, err := zstd.NewReader(recorder.Body)
					Expect(err).NotTo(HaveOccurred())
					defer tarByteReader.Close()
					err = tarfs.Extract(tarByteReader, unpackedDir)
					Expect(err).NotTo(HaveOccurred())
					fileInfo, err := os.Stat(filepath.Join(unpackedDir, "other-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeFalse())
					Expect(fileInfo.Size()).To(Equal(int64(len("other-file-content"))))

					contents, err := ioutil.ReadFile(filepath.Join(unpackedDir, "other-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("other-file-content"))

					dirInfo, err := os.Stat(filepath.Join(unpackedDir, "sub"))
					Expect(err).NotTo(HaveOccurred())
					Expect(dirInfo.IsDir()).To(BeTrue())

					fileInfo, err = os.Stat(filepath.Join(unpackedDir, "sub/some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fileInfo.IsDir()).To(BeFalse())
					Expect(fileInfo.Size()).To(Equal(int64(len("some-file-content"))))

					contents, err = ioutil.ReadFile(filepath.Join(unpackedDir, "sub/some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(string(contents)).To(Equal("some-file-content"))
				})
			})
		})

		Context("when using an unsupported encoding", func() {
			It("returns 400 when err is UnsupportedEncodingError", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "dest-path"), nil)
				request.Header.Set("Accept-Encoding", "unsupported")
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(400))
			})
		})

		It("returns 404 when volume is not found", func() {
			request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out", "invalid-handle"), nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(404))
		})
	})

	Describe("updating a volume", func() {
		It("can have it's properties updated", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				Properties: baggageclaim.VolumeProperties{
					"property-name": "property-val",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-name=property-val", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(1))

			err = json.NewEncoder(body).Encode(baggageclaim.PropertyRequest{
				Value: "other-val",
			})
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/properties/property-name", volumes[0].Handle), body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNoContent))
			Expect(recorder.Body.String()).To(BeEmpty())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-name=other-val", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())

			Expect(volumes).To(HaveLen(1))
		})

	})

	Describe("destroying a volume", func() {
		It("can be destroyed", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			var createdVolume volume.Volume
			err = json.NewDecoder(recorder.Body).Decode(&createdVolume)
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, err = http.NewRequest("DELETE", fmt.Sprintf("/volumes/%s", createdVolume.Handle), nil)
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNoContent))
			Expect(recorder.Body.String()).To(BeEmpty())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(0))
		})
	})

	Describe("destroying volumes", func() {
		It("can be destroyed", func() {
			for i := 1; i < 3; i++ {
				body := &bytes.Buffer{}
				err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
					Handle: fmt.Sprintf("some-handle-%d", i),
					Strategy: encStrategy(map[string]string{
						"type": "empty",
					}),
				})
				Expect(err).NotTo(HaveOccurred())

				recorder := httptest.NewRecorder()
				request, err := http.NewRequest("POST", "/volumes", body)
				Expect(err).NotTo(HaveOccurred())
				handler.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(201))
			}

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("GET", "/volumes", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err := json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(2))

			body := &bytes.Buffer{}
			err = json.NewEncoder(body).Encode([]string{"some-handle-1", "some-handle-2", "some-handle-3"})
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("DELETE", "/volumes/destroy", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(204))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes1 volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes1)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes1).To(HaveLen(0))
		})
	})

	Describe("creating a volume", func() {
		var (
			recorder *httptest.ResponseRecorder
			body     io.ReadWriter
		)

		JustBeforeEach(func() {
			recorder = httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)

			handler.ServeHTTP(recorder, request)
		})

		Context("when there are properties given", func() {
			var properties baggageclaim.VolumeProperties

			Context("with valid properties", func() {
				BeforeEach(func() {
					properties = baggageclaim.VolumeProperties{
						"property-name": "property-value",
					}

					body = &bytes.Buffer{}
					_ = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "empty",
						}),
						Properties: properties,
					})
				})

				It("creates the properties file", func() {
					var response volume.Volume
					err := json.NewDecoder(recorder.Body).Decode(&response)
					Expect(err).NotTo(HaveOccurred())

					propertiesPath := filepath.Join(volumeDir, "live", response.Handle, "properties.json")
					Expect(propertiesPath).To(BeAnExistingFile())

					propertiesContents, err := ioutil.ReadFile(propertiesPath)
					Expect(err).NotTo(HaveOccurred())

					var storedProperties baggageclaim.VolumeProperties
					err = json.Unmarshal(propertiesContents, &storedProperties)
					Expect(err).NotTo(HaveOccurred())

					Expect(storedProperties).To(Equal(properties))
				})

				It("returns the properties in the response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"property-name":"property-value"`))
				})
			})
		})

		Context("when there are no properties given", func() {
			BeforeEach(func() {
				body = &bytes.Buffer{}
				_ = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
					Handle: "some-handle",
					Strategy: encStrategy(map[string]string{
						"type": "empty",
					}),
				})
			})

			It("writes a nice JSON", func() {
				Expect(recorder.Body).To(ContainSubstring(`"path":`))
				Expect(recorder.Body).To(ContainSubstring(`"handle":"some-handle"`))
			})

			Context("when handle is not provided in request", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					_ = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Strategy: encStrategy(map[string]string{
							"type": "empty",
						}),
					})
				})

				It("generates a handle", func() {
					Expect(recorder.Body).To(ContainSubstring(`"path":`))
					uuidV4Regexp := `[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}`
					Expect(recorder.Body).To(MatchRegexp(`"handle":"` + uuidV4Regexp + `"`))
				})
			})

			Context("when invalid JSON is submitted", func() {
				BeforeEach(func() {
					body = bytes.NewBufferString("{{{{{{")
				})

				It("returns a 400 Bad Request response", func() {
					Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when no strategy is submitted", func() {
				BeforeEach(func() {
					body = bytes.NewBufferString("{}")
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when an unrecognized strategy is submitted", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					_ = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "grime",
						}),
					})
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow but not parent volume is provided", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					_ = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "cow",
						}),
					})
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow and the parent volume does not exist", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					_ = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Strategy: encStrategy(map[string]string{
							"type":   "cow",
							"volume": "#pain",
						}),
					})
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})
		})
	})

	Describe("creating a volume asynchronously", func() {
		It("returns a volume future", func() {
			body := &bytes.Buffer{}
			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle-1",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest("POST", "/volumes-async", body)
			Expect(err).NotTo(HaveOccurred())

			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(201))

			var volumeFuture baggageclaim.VolumeFutureResponse
			err = json.NewDecoder(recorder.Body).Decode(&volumeFuture)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeFuture.Handle).To(Equal("some-handle-1"))
		})

		Context("after creating a future", func() {
			It("returns a volume as soon as it has been created", func() {
				body := &bytes.Buffer{}
				err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
					Handle: "some-handle-2",
					Strategy: encStrategy(map[string]string{
						"type": "empty",
					}),
				})
				Expect(err).NotTo(HaveOccurred())

				recorder := httptest.NewRecorder()
				request, err := http.NewRequest("POST", "/volumes-async", body)
				Expect(err).NotTo(HaveOccurred())

				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(201))

				var volumeFuture baggageclaim.VolumeFutureResponse
				err = json.NewDecoder(recorder.Body).Decode(&volumeFuture)
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				request, err = http.NewRequest("GET", "/volumes-async/"+volumeFuture.Handle, nil)
				Expect(err).NotTo(HaveOccurred())

				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(SatisfyAny(Equal(204), Equal(200)))

				Eventually(func() *httptest.ResponseRecorder {
					recorder = httptest.NewRecorder()
					request, err := http.NewRequest("GET", "/volumes-async/"+volumeFuture.Handle, nil)
					Expect(err).NotTo(HaveOccurred())

					handler.ServeHTTP(recorder, request)

					return recorder
				}).Should(SatisfyAll(
					WithTransform(func(recorder *httptest.ResponseRecorder) int { return recorder.Code }, Equal(200)),
					WithTransform(func(recorder *httptest.ResponseRecorder) baggageclaim.VolumeResponse {
						var volumeResponse baggageclaim.VolumeResponse
						err := json.NewDecoder(recorder.Body).Decode(&volumeResponse)
						Expect(err).NotTo(HaveOccurred())
						return volumeResponse
					}, Equal(baggageclaim.VolumeResponse{
						Handle:     "some-handle-2",
						Path:       filepath.Join(volumeDir, "live", "some-handle-2", "volume"),
						Properties: nil,
					})),
				))
			})

			It("can be canceled", func() {
				body := &bytes.Buffer{}
				err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
					Handle: "some-handle-3",
					Strategy: encStrategy(map[string]string{
						"type": "empty",
					}),
				})
				Expect(err).NotTo(HaveOccurred())

				recorder := httptest.NewRecorder()
				request, err := http.NewRequest("POST", "/volumes-async", body)
				Expect(err).NotTo(HaveOccurred())

				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(201))

				var volumeFuture baggageclaim.VolumeFutureResponse
				err = json.NewDecoder(recorder.Body).Decode(&volumeFuture)
				Expect(err).NotTo(HaveOccurred())

				recorder = httptest.NewRecorder()
				request, err = http.NewRequest("DELETE", "/volumes-async/"+volumeFuture.Handle, nil)
				Expect(err).NotTo(HaveOccurred())

				handler.ServeHTTP(recorder, request)

				Expect(recorder.Code).To(Equal(204))
			})
		})
	})

	Describe("p2p stream tar out of a volume", func() {
		var (
			myVolume    volume.Volume
			encoding    string
			otherWorker *httptest.Server
		)

		JustBeforeEach(func() {
			// Init a fake remote worker
			otherWorker = httptest.NewServer(handler)

			// Create a volume to stream out
			body := &bytes.Buffer{}
			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
			})
			Expect(err).NotTo(HaveOccurred())
			request, err := http.NewRequest("POST", "/volumes", body)
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusCreated))

			err = json.NewDecoder(recorder.Body).Decode(&myVolume)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if otherWorker != nil {
				otherWorker.Close()
			}
		})

		It("returns an error when source path is invalid", func() {
			streamInP2pURL := fmt.Sprintf("%s/volumes/%s/stream-in?path=dest-path", otherWorker.URL, myVolume.Handle)
			request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-p2p-out?path=%s&streamInURL=%s&encoding=gzip", myVolume.Handle, "bogus-path", streamInP2pURL), nil)
			request.Header.Set("Accept-Encoding", string(baggageclaim.GzipEncoding))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(200)) // status code should always be 200, error is in body
			Expect(recorder.Body.String()).To(MatchRegexp("failed to compress source volume: .*: (no such file or directory|The system cannot find the file specified)"))
		})

		Context("when streaming a file", func() {
			JustBeforeEach(func() {
				// Create a file in the volume.
				filePath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "some-file")
				err := ioutil.WriteFile(filePath, []byte("some-file-content"), os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				streamInP2pURL := fmt.Sprintf("%s/volumes/%s/stream-in?path=dest-path", otherWorker.URL, myVolume.Handle)
				streamP2pOutRequest, _ := http.NewRequest(
					"PUT",
					fmt.Sprintf("/volumes/%s/stream-p2p-out?path=%s&streamInURL=%s&encoding=%s",
						myVolume.Handle, "some-file", streamInP2pURL, encoding),
					nil)
				streamP2pOutRecorder := httptest.NewRecorder()
				handler.ServeHTTP(streamP2pOutRecorder, streamP2pOutRequest)
				Expect(streamP2pOutRecorder.Code).To(Equal(200))
				Expect(strings.TrimSpace(streamP2pOutRecorder.Body.String())).To(Equal("ok"))

				destContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
				Expect(destContentsPath).To(BeAnExistingFile())
				Expect(ioutil.ReadFile(destContentsPath)).To(Equal([]byte("some-file-content")))
			})

			Context("when using gzip encoding", func() {
				BeforeEach(func() {
					encoding = string(baggageclaim.GzipEncoding)
				})

				It("should succeed", func() {
					// tests are in JustBeforeEach
				})
			})

			Context("when using zstd encoding", func() {
				BeforeEach(func() {
					encoding = string(baggageclaim.ZstdEncoding)
				})

				It("should succeed", func() {
					// tests are in JustBeforeEach
				})
			})
		})

		Context("when streaming a directory", func() {
			JustBeforeEach(func() {
				// Create a dir and a few files under the dir in the volume.
				path := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "some-dir")
				err := os.MkdirAll(filepath.Join(path, "sub"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(filepath.Join(path, "sub", "some-file"), []byte("some-file-content"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
				err = ioutil.WriteFile(filepath.Join(path, "other-file"), []byte("other-file-content"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				streamInP2pURL := fmt.Sprintf("%s/volumes/%s/stream-in?path=dest-path", otherWorker.URL, myVolume.Handle)
				streamP2pOutRequest, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-p2p-out?path=%s&streamInURL=%s&encoding=%s", myVolume.Handle, "some-dir", streamInP2pURL, encoding), nil)
				streamP2pOutRecorder := httptest.NewRecorder()
				handler.ServeHTTP(streamP2pOutRecorder, streamP2pOutRequest)
				Expect(streamP2pOutRecorder.Code).To(Equal(200))
				Expect(strings.TrimSpace(streamP2pOutRecorder.Body.String())).To(Equal("ok"))

				someContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "sub", "some-file")
				Expect(someContentsPath).To(BeAnExistingFile())
				Expect(ioutil.ReadFile(someContentsPath)).To(Equal([]byte("some-file-content")))

				otherContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "other-file")
				Expect(otherContentsPath).To(BeAnExistingFile())
				Expect(ioutil.ReadFile(otherContentsPath)).To(Equal([]byte("other-file-content")))
			})

			Context("when using gzip encoding", func() {
				BeforeEach(func() {
					encoding = string(baggageclaim.GzipEncoding)
				})
				It("should succeed", func() {
					// real tests are in JustBeforeEach
				})
			})

			Context("when using zstd encoding", func() {
				BeforeEach(func() {
					encoding = string(baggageclaim.ZstdEncoding)
				})
				It("should succeed", func() {
					// real tests are in JustBeforeEach
				})
			})
		})
	})
})

func encStrategy(strategy map[string]string) *json.RawMessage {
	bytes, err := json.Marshal(strategy)
	Expect(err).NotTo(HaveOccurred())

	msg := json.RawMessage(bytes)

	return &msg
}
