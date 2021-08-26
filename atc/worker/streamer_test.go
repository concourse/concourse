package worker_test

import (
	"context"
	"io/ioutil"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/runtime/runtimetest"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/gardenruntime"
	grt "github.com/concourse/concourse/atc/worker/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/atc/worker/workertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streamer", func() {
	Test("stream volume through ATC", func() {
		content := runtimetest.VolumeContent{
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

		streamer := scenario.Streamer(worker.P2PConfig{
			Enabled: false,
		})

		ctx := context.Background()
		src := scenario.WorkerVolume("src-worker", "src")
		dst := scenario.WorkerVolume("dst-worker", "dst")

		err := streamer.Stream(ctx, src, dst)
		Expect(err).ToNot(HaveOccurred())

		Expect(baggageclaimVolume(dst)).To(grt.HaveContent(content))
	})

	Test("stream artifact through ATC", func() {
		artifact := runtimetest.Artifact{
			Content: runtimetest.VolumeContent{
				"file1":        {Data: []byte("content 1")},
				"folder/file2": {Data: []byte("content 2")},
			},
		}
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("dst-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("dst"),
					),
			),
		)

		streamer := scenario.Streamer(worker.P2PConfig{
			Enabled: false,
		})

		ctx := context.Background()
		dst := scenario.WorkerVolume("dst-worker", "dst")

		err := streamer.Stream(ctx, artifact, dst)
		Expect(err).ToNot(HaveOccurred())

		Expect(baggageclaimVolume(dst)).To(grt.HaveContent(artifact.Content))
	})

	Test("stream a resource cache volume", func() {
		atc.EnableCacheStreamedVolumes = true

		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("src-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("src"),
					),
				grt.NewWorker("dst-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("dst"),
					),
			),
		)

		streamer := scenario.Streamer(worker.P2PConfig{
			Enabled: false,
		})

		ctx := context.Background()
		src := scenario.WorkerVolume("src-worker", "src")
		dst := scenario.WorkerVolume("dst-worker", "dst")

		By("setting the dst volume as privileged", func() {
			err := baggageclaimVolume(dst).SetPrivileged(true)
			Expect(err).ToNot(HaveOccurred())
		})

		var resourceCache db.ResourceCache
		By("initializing src as a resource cache", func() {
			resourceCache = scenario.FindOrCreateResourceCache("src-worker")
			err := src.InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())
		})

		err := streamer.Stream(ctx, src, dst)
		Expect(err).ToNot(HaveOccurred())

		By("validating the volume was marked as a resource cache on the dst worker", func() {
			volume, found, err := scenario.DBBuilder.VolumeRepo.FindResourceCacheVolume("dst-worker", resourceCache)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(volume.Handle()).To(Equal(dst.Handle()))
		})

		By("validating the volume was marked as non-privileged", func() {
			// This test is specific to the gardenruntime - however, it's being
			// used to ensure we shell out to the runtime-specific
			// `InitializeResourceCache` method rather than on the DB volume
			// directly, since that could lead to subtle bugs.
			Expect(baggageclaimVolume(dst).Spec.Privileged).To(BeFalse(), "should have called the runtime specific InitializeResourceCache method")
		})
	})

	Test("does not cache streamed volumes when setting is disabled", func() {
		atc.EnableCacheStreamedVolumes = false

		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("src-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("src"),
					),
				grt.NewWorker("dst-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("dst"),
					),
			),
		)

		streamer := scenario.Streamer(worker.P2PConfig{
			Enabled: false,
		})

		ctx := context.Background()
		src := scenario.WorkerVolume("src-worker", "src")
		dst := scenario.WorkerVolume("dst-worker", "dst")

		var resourceCache db.ResourceCache
		By("initializing src as a resource cache", func() {
			resourceCache = scenario.FindOrCreateResourceCache("src-worker")
			err := src.InitializeResourceCache(logger, resourceCache)
			Expect(err).ToNot(HaveOccurred())
		})

		err := streamer.Stream(ctx, src, dst)
		Expect(err).ToNot(HaveOccurred())

		By("validating the volume was NOT marked as a resource cache on the dst worker", func() {
			_, found, err := scenario.DBBuilder.VolumeRepo.FindResourceCacheVolume("dst-worker", resourceCache)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Test("P2P stream between workers", func() {
		content := runtimetest.VolumeContent{
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

		streamer := scenario.Streamer(worker.P2PConfig{
			Enabled: true,
		})

		ctx := context.Background()
		src := scenario.WorkerVolume("src-worker", "src")
		dst := scenario.WorkerVolume("dst-worker", "dst")

		err := streamer.Stream(ctx, src, dst)
		Expect(err).ToNot(HaveOccurred())

		Expect(baggageclaimVolume(dst)).To(grt.HaveContent(content))
	})

	Test("stream file from volume", func() {
		content := runtimetest.VolumeContent{
			"file":        {Data: []byte("content 1")},
			"folder/file": {Data: []byte("content 2")},
		}
		scenario := Setup(
			workertest.WithWorkers(
				grt.NewWorker("src-worker").
					WithVolumesCreatedInDBAndBaggageclaim(
						grt.NewVolume("src").WithContent(content),
					),
			),
		)

		streamer := scenario.Streamer(worker.P2PConfig{
			Enabled: false,
		})

		ctx := context.Background()
		src := scenario.WorkerVolume("src-worker", "src")

		stream, err := streamer.StreamFile(ctx, src, "folder/file")
		Expect(err).ToNot(HaveOccurred())

		fileContent, err := ioutil.ReadAll(stream)
		Expect(err).ToNot(HaveOccurred())

		Expect(fileContent).To(Equal([]byte("content 2")))
	})

	Test("stream file from artifact", func() {
		artifact := runtimetest.Artifact{
			Content: runtimetest.VolumeContent{
				"file":        {Data: []byte("content 1")},
				"folder/file": {Data: []byte("content 2")},
			},
		}
		streamer := Setup().Streamer(worker.P2PConfig{
			Enabled: false,
		})

		ctx := context.Background()
		stream, err := streamer.StreamFile(ctx, artifact, "folder/file")
		Expect(err).ToNot(HaveOccurred())

		fileContent, err := ioutil.ReadAll(stream)
		Expect(err).ToNot(HaveOccurred())

		Expect(fileContent).To(Equal([]byte("content 2")))
	})
})

func baggageclaimVolume(volume runtime.Volume) *grt.Volume {
	grVolume, ok := volume.(gardenruntime.Volume)
	Expect(ok).To(BeTrue(), "must be called on a gardenruntime.Volume")

	bcVolume := grVolume.BaggageclaimVolume().(*grt.Volume)
	return bcVolume
}
