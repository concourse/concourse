//go:build linux

package baggageclaimcmd_test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/v8/atc/compression"
	"github.com/concourse/concourse/v8/worker/baggageclaim"
	"github.com/concourse/concourse/v8/worker/baggageclaim/baggageclaimcmd"
	"github.com/concourse/concourse/v8/worker/baggageclaim/uidgid"
	"github.com/concourse/concourse/v8/worker/baggageclaim/volume"
	"github.com/concourse/concourse/v8/worker/baggageclaim/volume/driver"
	bespec "github.com/concourse/concourse/v8/worker/runtime/spec"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streaming volume ownership", func() {
	for _, mode := range []struct {
		name                  string
		privilegedMode        bespec.PrivilegedMode
		disableUserNamespaces bool
	}{
		{name: "full", privilegedMode: bespec.FullPrivilegedMode},
		{name: "fuse-only", privilegedMode: bespec.FUSEOnlyPrivilegedMode},
		{name: "ignore", privilegedMode: bespec.IgnorePrivilegedMode},
		{name: "disabled user namespaces", privilegedMode: bespec.FUSEOnlyPrivilegedMode, disableUserNamespaces: true},
	} {
		Describe(mode.name, func() {
			for _, encoding := range []compression.Compression{
				compression.NewNoCompression(),
				compression.NewGzipCompression(),
				compression.NewZstdCompression(),
				compression.NewS2Compression(),
			} {
				Describe(string(encoding.Encoding()), func() {
					DescribeTable("preserves root and non-root ownership across workers", func(sourcePrivileged, destinationPrivileged, p2p bool) {
						logger := lagertest.NewTestLogger("streaming-ownership")
						cmd := baggageclaimcmd.BaggageclaimCommand{
							PrivilegedMode:        mode.privilegedMode,
							DisableUserNamespaces: mode.disableUserNamespaces,
						}
						privilegedNamespacer, unprivilegedNamespacer := cmd.SelectNamespacers(logger)
						volumeDir, err := os.MkdirTemp("", "streaming-ownership")
						Expect(err).NotTo(HaveOccurred())
						DeferCleanup(func() { Expect(os.RemoveAll(volumeDir)).To(Succeed()) })
						Expect(os.Chmod(volumeDir, 0755)).To(Succeed())
						fs, err := volume.NewFilesystem(logger, &driver.NaiveDriver{}, volumeDir)
						Expect(err).NotTo(HaveOccurred())
						repo := volume.NewRepository(fs, volume.NewLockManager(), privilegedNamespacer, unprivilegedNamespacer, mode.privilegedMode != bespec.FullPrivilegedMode)
						ctx := context.Background()
						source, err := repo.CreateVolume(ctx, "source", volume.EmptyStrategy{}, volume.VolumeOpts{Privileged: sourcePrivileged})
						Expect(err).NotTo(HaveOccurred())
						destination, err := repo.CreateVolume(ctx, "destination", volume.EmptyStrategy{}, volume.VolumeOpts{Privileged: destinationPrivileged})
						Expect(err).NotTo(HaveOccurred())

						sourceRootID, destinationRootID := 0, 0
						if !mode.disableUserNamespaces {
							if !sourcePrivileged || mode.privilegedMode != bespec.FullPrivilegedMode {
								sourceRootID = min(uidgid.MustGetMaxValidUID(), uidgid.MustGetMaxValidGID())
							}
							if !destinationPrivileged || mode.privilegedMode != bespec.FullPrivilegedMode {
								destinationRootID = min(uidgid.MustGetMaxValidUID(), uidgid.MustGetMaxValidGID())
							}
						}
						for name, owner := range map[string]int{"root-file": sourceRootID, "user-file": 123} {
							path := filepath.Join(source.Path, name)
							Expect(os.WriteFile(path, []byte(name), 0644)).To(Succeed())
							Expect(os.Chown(path, owner, owner)).To(Succeed())
						}

						if p2p {
							server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
								_, streamErr := repo.StreamIn(request.Context(), destination.Handle, ".", baggageclaim.Encoding(request.Header.Get("Content-Encoding")), 0, request.Body)
								if streamErr != nil {
									http.Error(w, fmt.Sprintf(`{"error":%q}`, streamErr.Error()), http.StatusBadRequest)
									return
								}
								w.WriteHeader(http.StatusNoContent)
							}))
							DeferCleanup(server.Close)
							_, err := repo.StreamP2pOut(ctx, source.Handle, ".", encoding.Encoding(), server.URL)
							Expect(err).NotTo(HaveOccurred())
						} else {
							var archive bytes.Buffer
							Expect(repo.StreamOut(ctx, source.Handle, ".", encoding.Encoding(), &archive)).To(Succeed())
							reader, err := encoding.NewReader(io.NopCloser(bytes.NewReader(archive.Bytes())))
							Expect(err).NotTo(HaveOccurred())
							DeferCleanup(func() { Expect(reader.Close()).To(Succeed()) })
							tarReader := tar.NewReader(reader)
							owners := map[string]int{}
							for {
								header, err := tarReader.Next()
								if err == io.EOF {
									break
								}
								Expect(err).NotTo(HaveOccurred())
								owners[filepath.Clean(header.Name)] = header.Uid
								Expect(header.Gid).To(Equal(header.Uid))
							}
							Expect(owners).To(HaveKeyWithValue("root-file", 0))
							Expect(owners).To(HaveKeyWithValue("user-file", 123))
							badStream, err := repo.StreamIn(ctx, destination.Handle, ".", encoding.Encoding(), 0, &archive)
							Expect(err).NotTo(HaveOccurred())
							Expect(badStream).To(BeFalse())
						}

						for name, owner := range map[string]int{"root-file": destinationRootID, "user-file": 123} {
							path := filepath.Join(destination.Path, name)
							contents, err := os.ReadFile(path)
							Expect(err).NotTo(HaveOccurred())
							Expect(string(contents)).To(Equal(name))
							info, err := os.Stat(path)
							Expect(err).NotTo(HaveOccurred())
							stat := info.Sys().(*syscall.Stat_t)
							Expect(stat.Uid).To(Equal(uint32(owner)))
							Expect(stat.Gid).To(Equal(uint32(owner)))
						}
					},
						Entry("unprivileged to unprivileged", false, false, false),
						Entry("privileged to unprivileged", true, false, false),
						Entry("unprivileged to privileged", false, true, false),
						Entry("privileged to privileged", true, true, false),
						Entry("P2P unprivileged to unprivileged", false, false, true),
						Entry("P2P privileged to unprivileged", true, false, true),
						Entry("P2P unprivileged to privileged", false, true, true),
						Entry("P2P privileged to privileged", true, true, true),
					)
				})
			}
		})
	}
})
