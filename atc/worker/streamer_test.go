package worker_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"time"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/atc/compression"
	"github.com/concourse/concourse/v8/atc/db"
	"github.com/concourse/concourse/v8/atc/runtime"
	"github.com/concourse/concourse/v8/atc/runtime/runtimetest"
	"github.com/concourse/concourse/v8/atc/worker"
	"github.com/concourse/concourse/v8/atc/worker/gardenruntime"
	grt "github.com/concourse/concourse/v8/atc/worker/gardenruntime/gardenruntimetest"
	"github.com/concourse/concourse/v8/atc/worker/workertest"
	"github.com/concourse/concourse/v8/tracing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
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
			err := baggageclaimVolume(dst).SetPrivileged(ctx, true)
			Expect(err).ToNot(HaveOccurred())
		})

		var resourceCache db.ResourceCache
		By("initializing src as a resource cache", func() {
			resourceCache = scenario.FindOrCreateResourceCache("src-worker")
			_, err := src.InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())
		})

		err := streamer.Stream(ctx, src, dst)
		Expect(err).ToNot(HaveOccurred())

		By("validating the volume was marked as a resource cache on the dst worker", func() {
			volume, found, err := scenario.DBBuilder.VolumeRepo.FindResourceCacheVolume("dst-worker", resourceCache, time.Now())
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
			_, err := src.InitializeResourceCache(ctx, resourceCache)
			Expect(err).ToNot(HaveOccurred())
		})

		err := streamer.Stream(ctx, src, dst)
		Expect(err).ToNot(HaveOccurred())

		By("validating the volume was NOT marked as a resource cache on the dst worker", func() {
			_, found, err := scenario.DBBuilder.VolumeRepo.FindResourceCacheVolume("dst-worker", resourceCache, time.Now())
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

	DescribeTable("P2P streaming group routing", func(srcGroup, dstGroup string, enabled, wantP2P bool) {
		content := runtimetest.VolumeContent{
			"file1":        {Data: []byte("content 1")},
			"folder/file2": {Data: []byte("content 2")},
		}
		scenario := Setup(workertest.WithWorkers(
			grt.NewWorker("src-worker").WithP2PStreamingGroup(srcGroup).
				WithVolumesCreatedInDBAndBaggageclaim(grt.NewVolume("src").WithContent(content)),
			grt.NewWorker("dst-worker").WithP2PStreamingGroup(dstGroup).
				WithVolumesCreatedInDBAndBaggageclaim(grt.NewVolume("dst")),
		))
		src := &trackedP2PVolume{P2PVolume: scenario.WorkerVolume("src-worker", "src").(runtime.P2PVolume)}
		dst := &trackedP2PVolume{P2PVolume: scenario.WorkerVolume("dst-worker", "dst").(runtime.P2PVolume)}

		err := scenario.Streamer(worker.P2PConfig{Enabled: enabled}).Stream(context.Background(), src, dst)
		Expect(err).ToNot(HaveOccurred())
		Expect(baggageclaimVolume(dst.P2PVolume)).To(grt.HaveContent(content))
		if wantP2P {
			Expect(src.p2pOutCalls).To(Equal(1))
			Expect(dst.p2pURLCalls).To(Equal(1))
			Expect(src.streamOutCalls).To(BeZero())
			Expect(dst.streamInCalls).To(BeZero())
		} else {
			Expect(src.p2pOutCalls).To(BeZero())
			Expect(dst.p2pURLCalls).To(BeZero())
			Expect(src.streamOutCalls).To(Equal(1))
			Expect(dst.streamInCalls).To(Equal(1))
		}
	},
		Entry("same named group streams directly", "group-a", "group-a", true, true),
		Entry("two ungrouped workers stream directly", "", "", true, true),
		Entry("different groups stream through ATC", "group-a", "group-b", true, false),
		Entry("group names are case sensitive", "group-a", "Group-a", true, false),
		Entry("grouped source and ungrouped destination stream through ATC", "group-a", "", true, false),
		Entry("ungrouped source and grouped destination stream through ATC", "", "group-a", true, false),
		Entry("disabled P2P streams through ATC even within a group", "group-a", "group-a", false, false),
	)

	Test("traces streams", func() {
		spanRecorder := new(tracetest.SpanRecorder)
		tracing.ConfigureTraceProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder)))
		defer func() { tracing.Configured = false }()

		endedSpan := func(name string) sdktrace.ReadOnlySpan {
			for _, span := range spanRecorder.Ended() {
				if span.Name() == name {
					return span
				}
			}
			Fail("no ended span named " + name)
			return nil
		}

		scenario := Setup(workertest.WithWorkers(
			grt.NewWorker("src-worker").
				WithVolumesCreatedInDBAndBaggageclaim(grt.NewVolume("src").WithContent(runtimetest.VolumeContent{
					"file1": {Data: []byte("content 1")},
				})),
			grt.NewWorker("dst-worker").
				WithVolumesCreatedInDBAndBaggageclaim(grt.NewVolume("dst")),
		))
		src := &trackedP2PVolume{P2PVolume: scenario.WorkerVolume("src-worker", "src").(runtime.P2PVolume)}
		dst := scenario.WorkerVolume("dst-worker", "dst")

		By("streaming P2P within the volume.P2pStreamOut span")
		err := scenario.Streamer(worker.P2PConfig{Enabled: true}).Stream(context.Background(), src, dst)
		Expect(err).ToNot(HaveOccurred())
		Expect(trace.SpanContextFromContext(src.p2pOutCtx)).To(Equal(endedSpan("volume.P2pStreamOut").SpanContext()))

		By("streaming through the ATC within the volume.StreamThroughATC span")
		err = scenario.Streamer(worker.P2PConfig{Enabled: false}).Stream(context.Background(), src, dst)
		Expect(err).ToNot(HaveOccurred())
		Expect(trace.SpanContextFromContext(src.streamOutCtx)).To(Equal(endedSpan("volume.StreamThroughATC").SpanContext()))
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

		defer stream.Close()

		fileContent, err := io.ReadAll(stream)
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

		defer stream.Close()

		fileContent, err := io.ReadAll(stream)
		Expect(err).ToNot(HaveOccurred())

		Expect(fileContent).To(Equal([]byte("content 2")))
	})

	Test("close the stream when the file cannot be read", func() {
		streamer := Setup().Streamer(worker.P2PConfig{
			Enabled: false,
		})

		By("closing it when the stream is not compressed")
		notCompressed := &closeTrackingArtifact{Reader: bytes.NewReader([]byte("not gzip"))}
		_, err := streamer.StreamFile(context.Background(), notCompressed, "file")
		Expect(err).To(HaveOccurred())
		Expect(notCompressed.closed).To(BeTrue())

		By("closing it when the stream holds no tar entry")
		compressed := new(bytes.Buffer)
		gzipWriter := gzip.NewWriter(compressed)
		_, err = gzipWriter.Write([]byte("not a tar"))
		Expect(err).ToNot(HaveOccurred())
		Expect(gzipWriter.Close()).To(Succeed())

		notTar := &closeTrackingArtifact{Reader: compressed}
		_, err = streamer.StreamFile(context.Background(), notTar, "file")
		Expect(err).To(HaveOccurred())
		Expect(notTar.closed).To(BeTrue())
	})
})

// closeTrackingArtifact streams out itself, so that a test can tell whether
// the stream was closed.
type closeTrackingArtifact struct {
	runtime.Artifact
	io.Reader
	closed bool
}

func (a *closeTrackingArtifact) StreamOut(context.Context, string, compression.Compression) (io.ReadCloser, error) {
	return a, nil
}

func (a *closeTrackingArtifact) Close() error {
	a.closed = true
	return nil
}

func baggageclaimVolume(volume runtime.Volume) *grt.Volume {
	grVolume, ok := volume.(gardenruntime.Volume)
	Expect(ok).To(BeTrue(), "must be called on a gardenruntime.Volume")

	bcVolume := grVolume.BaggageclaimVolume().(*grt.Volume)
	return bcVolume
}

// Count calls at the runtime boundary so successful content delivery cannot
// hide an unexpected fallback from P2P to ATC streaming.
type trackedP2PVolume struct {
	runtime.P2PVolume
	p2pOutCalls, p2pURLCalls, streamOutCalls, streamInCalls int

	p2pOutCtx, streamOutCtx context.Context
}

func (v *trackedP2PVolume) GetStreamInP2PURL(ctx context.Context, path string) (string, error) {
	v.p2pURLCalls++
	return v.P2PVolume.GetStreamInP2PURL(ctx, path)
}

func (v *trackedP2PVolume) StreamP2POut(ctx context.Context, path, url string, c compression.Compression) error {
	v.p2pOutCalls++
	v.p2pOutCtx = ctx
	return v.P2PVolume.StreamP2POut(ctx, path, url, c)
}

func (v *trackedP2PVolume) StreamOut(ctx context.Context, path string, c compression.Compression) (io.ReadCloser, error) {
	v.streamOutCalls++
	v.streamOutCtx = ctx
	return v.P2PVolume.StreamOut(ctx, path, c)
}

func (v *trackedP2PVolume) StreamIn(ctx context.Context, path string, c compression.Compression, limit float64, in io.Reader) error {
	v.streamInCalls++
	return v.P2PVolume.StreamIn(ctx, path, c, limit, in)
}
