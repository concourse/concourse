package worker_test

import (
	"context"
	"io/ioutil"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ArtifactStreamer", func() {
	It("streams files from an artifact", func() {
		artifact := &runtime.TaskArtifact{VolumeHandle: "output"}
		expectedContent := tarGzContent(file{"file.txt", []byte("some file")})
		vf := FakeVolumeFinder{Volumes: map[string]worker.Volume{
			"output": newVolumeWithContent(content{"file.txt": expectedContent}),
		}}

		streamer := worker.NewArtifactStreamer(vf, compression.NewGzipCompression())
		reader, err := streamer.StreamFileFromArtifact(context.Background(), artifact, "file.txt")
		Expect(err).ToNot(HaveOccurred())

		content, err := ioutil.ReadAll(reader)
		Expect(err).ToNot(HaveOccurred())
		Expect(content).To(Equal([]byte("some file")))
	})

	Context("when the artifact is not found", func() {
		It("errors", func() {
			artifact := &runtime.TaskArtifact{VolumeHandle: "missing_output"}
			vf := FakeVolumeFinder{}

			streamer := worker.NewArtifactStreamer(vf, compression.NewGzipCompression())
			_, err := streamer.StreamFileFromArtifact(context.Background(), artifact, "file.txt")
			Expect(err).To(MatchError(baggageclaim.ErrVolumeNotFound))
		})
	})
})
