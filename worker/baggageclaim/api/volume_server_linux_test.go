//go:build linux
// +build linux

package api_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"

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

		tempDir, err = ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
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

		privilegedNamespacer := &uidgid.UidNamespacer{
			Translator: uidgid.NewTranslator(uidgid.NewPrivilegedMapper()),
			Logger:     logger.Session("uid-namespacer"),
		}

		unprivilegedNamespacer := &uidgid.UidNamespacer{
			Translator: uidgid.NewTranslator(uidgid.NewUnprivilegedMapper()),
			Logger:     logger.Session("uid-namespacer"),
		}

		repo := volume.NewRepository(
			fs,
			volume.NewLockManager(),
			privilegedNamespacer,
			unprivilegedNamespacer,
		)

		strategerizer := volume.NewStrategerizer()

		re := regexp.MustCompile("eth0")
		handler, err = api.NewHandler(logger, strategerizer, repo, re, 4, 7766)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir + "/*")
		Expect(err).NotTo(HaveOccurred())
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

			Context("when volume is not privileged", func() {
				BeforeEach(func() {
					isPrivileged = false
				})

				It("namespaces volume path", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tgzBuffer)
					request.Header.Set("Content-Encoding", "gzip")
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(204))

					tarInfoPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
					Expect(tarInfoPath).To(BeAnExistingFile())

					stat, err := os.Stat(tarInfoPath)
					Expect(err).ToNot(HaveOccurred())

					maxUID := uidgid.MustGetMaxValidUID()
					maxGID := uidgid.MustGetMaxValidGID()

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(maxUID)))
					Expect(sysStat.Gid).To(Equal(uint32(maxGID)))
				})
			})

			Context("when volume privileged", func() {
				BeforeEach(func() {
					isPrivileged = true
				})

				It("namespaces volume path", func() {
					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tgzBuffer)
					request.Header.Set("Content-Encoding", "gzip")
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(204))

					tarInfoPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
					Expect(tarInfoPath).To(BeAnExistingFile())

					stat, err := os.Stat(tarInfoPath)
					Expect(err).ToNot(HaveOccurred())

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(0)))
					Expect(sysStat.Gid).To(Equal(uint32(0)))
				})
			})
		})
	})
})
