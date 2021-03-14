package worker_test

import (
	"context"
	"testing/fstest"

	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/gardenruntime"
	grt "github.com/concourse/concourse/atc/worker/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/atc/worker/workertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streamer", func() {
	Test("stream through ATC", func() {
		content := fstest.MapFS{
			"file1":        {Data: []byte("content 1")},
			"folder/file2": {Data: []byte("content 2")},
		}
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("src-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("src").WithContent(content),
					),
				grt.NewWorker("dst-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("dst"),
					),
			),
		)

		streamer := worker.Streamer{
			Compression: compression.NewGzipCompression(),
		}

		ctx := context.Background()
		src := scenario.WorkerVolume("src-worker", "src")
		dst := scenario.WorkerVolume("dst-worker", "dst")

		err := streamer.Stream(ctx, "src-worker", src, dst)
		Expect(err).ToNot(HaveOccurred())

		Expect(baggageclaimVolume(dst)).To(grt.HaveContent(content))
	})

	Test("P2P stream between workers", func() {
		content := fstest.MapFS{
			"file1":        {Data: []byte("content 1")},
			"folder/file2": {Data: []byte("content 2")},
		}
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("src-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("src").WithContent(content),
					),
				grt.NewWorker("dst-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("dst"),
					),
			),
		)

		streamer := worker.Streamer{
			Compression:        compression.NewGzipCompression(),
			EnableP2PStreaming: true,
		}

		ctx := context.Background()
		src := scenario.WorkerVolume("src-worker", "src")
		dst := scenario.WorkerVolume("dst-worker", "dst")

		err := streamer.Stream(ctx, "src-worker", src, dst)
		Expect(err).ToNot(HaveOccurred())

		Expect(baggageclaimVolume(dst)).To(grt.HaveContent(content))
	})
})

func baggageclaimVolume(volume runtime.Volume) *grt.Volume {
	grVolume, ok := volume.(gardenruntime.Volume)
	Expect(ok).To(BeTrue(), "must be called on a gardenruntime.Volume")

	bcVolume := grVolume.BaggageclaimVolume().(*grt.Volume)
	return bcVolume
}
