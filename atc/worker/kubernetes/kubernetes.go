package kubernetes

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/handles"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/kubernetes/backend"
	log "github.com/sirupsen/logrus"
)

// TODO rename this to `Client`
//
type Kubernetes struct {
	be *backend.Backend
	wf db.WorkerFactory
	rf resource.ResourceFactory
	cs handles.Syncer
}

func NewClient(
	be *Backend,
	dbWorkerFactory db.WorkerFactory,
	resourceFactory resource.ResourceFactory,
	containerSyncer handles.Syncer,
) (k Kubernetes) {
	k = Kubernetes{
		be: be,
		wf: dbWorkerFactory,
		rf: resourceFactory,
	}

	return
}

var _ worker.Client = Kubernetes{}

func (k Kubernetes) RunTaskStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec worker.ContainerSpec,
	workerSpec worker.WorkerSpec,
	strategy worker.ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	imageFetcherSpec worker.ImageFetcherSpec,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	lockFactory lock.LockFactory,
) (result worker.TaskResult, err error) {
	container, err := k.findOrCreateContainer(
		owner,
		containerMetadata,
		containerSpec,
	)
	if err != nil {
		err = fmt.Errorf("find or create container: %w", err)
		return
	}

	stdin := bytes.NewBuffer(nil)
	processIO := garden.ProcessIO{
		Stdout: processSpec.StdoutWriter,
		Stderr: processSpec.StderrWriter,
		Stdin:  stdin,
	}

	_, err = container.Run(
		garden.ProcessSpec{
			ID: "task",

			Path: processSpec.Path,
			Args: processSpec.Args,

			Dir: path.Join(containerMetadata.WorkingDirectory, processSpec.Dir),

			// Guardian sets the default TTY window size to width: 80, height: 24,
			// which creates ANSI control sequences that do not work with other window sizes
			TTY: &garden.TTYSpec{
				WindowSize: &garden.WindowSize{Columns: 500, Rows: 500},
			},
		},
		processIO,
	)

	result = worker.TaskResult{
		ExitStatus: 0, // TODO not have this hardcoded
	}

	return
}

func (k Kubernetes) RunGetStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec worker.ContainerSpec,
	workerSpec worker.WorkerSpec,
	strategy worker.ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	fetcherSpec worker.ImageFetcherSpec,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resourceCache db.UsedResourceCache,
	resource resource.Resource,
) (result worker.GetResult, err error) {

	// tell that it's starting.
	//
	eventDelegate.Starting(logger)

	// register that we have outputs (so that we have a mount for it, in the
	// right place).
	//
	containerSpec.Outputs = map[string]string{
		"resource": processSpec.Args[0],
	}

	// TODO certs?
	//
	container, err := k.findOrCreateContainer(
		owner,
		containerMetadata,
		containerSpec,
	)
	if err != nil {
		err = fmt.Errorf("find or create container: %w", err)
		return
	}

	vr, err := resource.Get(ctx, processSpec, container)
	if err != nil {
		err = fmt.Errorf("get: %w", err)
		return
	}

	// TODO handle `ErrResourceScriptFailed` (exit status != 0)
	//
	result = worker.GetResult{
		ExitStatus:    0,
		VersionResult: vr,
		GetArtifact: runtime.GetArtifact{
			VolumeHandle: (PodArtifact{
				Pod:    container.Handle(),
				Ip:     container.IP(),
				Handle: "resource", // TODO [cc] fix this
			}).String(),
		},
	}

	return
}

func (k Kubernetes) RunCheckStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec worker.ContainerSpec,
	workerSpec worker.WorkerSpec,
	strategy worker.ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	resourceTypes atc.VersionedResourceTypes,
	timeout time.Duration,
	checkable resource.Resource,
) ([]atc.Version, error) {
	container, err := k.findOrCreateContainer(
		owner,
		containerMetadata,
		containerSpec,
	)
	if err != nil {
		return nil, fmt.Errorf("find or create container: %w", err)
	}

	result, err := checkable.Check(
		context.Background(),
		runtime.ProcessSpec{
			Path: "/opt/resource/check",
		},
		container,
	)
	if err != nil {
		return nil, fmt.Errorf("checking: %w", err)
	}

	return result, nil
}

func (k Kubernetes) RunPutStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec worker.ContainerSpec,
	workerSpec worker.WorkerSpec,
	strategy worker.ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	imageFetcherSpec worker.ImageFetcherSpec,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	resource resource.Resource,
) (result worker.PutResult, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (k Kubernetes) FindContainer(
	logger lager.Logger, teamID int, handle string,
) (
	container worker.Container, found bool, err error,
) {
	return
}

func (k Kubernetes) FindVolume(
	logger lager.Logger,
	teamID int,
	handle string,
) (vol worker.Volume, found bool, err error) {
	return
}
func (k Kubernetes) CreateVolume(
	logger lager.Logger,
	vSpec worker.VolumeSpec,
	wSpec worker.WorkerSpec,
	volumeType db.VolumeType,
) (vol worker.Volume, err error) {
	return
}
func (k Kubernetes) StreamFileFromArtifact(
	ctx context.Context,
	logger lager.Logger,
	artifact runtime.Artifact,
	filePath string,
) (rc io.ReadCloser, err error) {
	artf := UnmarshalPodArtifact(artifact.ID())

	pod, err := k.be.Lookup(artf.Pod)
	if err != nil {
		err = fmt.Errorf("lookup: %w", err)
		return
	}

	// TODO [cc] make this conditional
	// (if `in-cluster`, then there's no need for proxy'in)
	//
	forwardingSess, port, err := pod.PortForward("7788")
	if err != nil {
		err = fmt.Errorf("port forward: %w", err)
		return
	}

	defer forwardingSess.Close()

	uri := "http://localhost:" + port + "/volumes/" + artf.Handle + "/stream-out"

	rc, err = fetchFile(ctx, uri, filePath)
	if err != nil {
		err = fmt.Errorf("fetch file: %w", err)
		return
	}

	return
}

type readCloser struct {
	reader io.Reader
	closer io.Closer
}

func (r readCloser) Close() error {
	return r.closer.Close()
}

func (r readCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func fetchFile(ctx context.Context, uri, fpath string) (rc io.ReadCloser, err error) {
	log.Info("uri:" + uri)

	req, err := http.NewRequest("PUT", uri, nil)
	if err != nil {
		err = fmt.Errorf("new req: %w", err)
		return
	}

	req.Header.Set("Accept-Encoding", "gzip")

	req.URL.RawQuery = url.Values{"path": []string{fpath}}.Encode()
	if err != nil {
		err = fmt.Errorf("query encode: %w", err)
		return
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("do req: %w", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(resp.Body)
		err = fmt.Errorf("not ok: %s - %s", resp.Status, string(b))
		return
	}

	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		err = fmt.Errorf("gzip reader: %w", err)
		return
	}
	defer gzipReader.Close() // should I ?

	tarReader := tar.NewReader(gzipReader)
	_, err = tarReader.Next()
	if err != nil {
		err = fmt.Errorf("tar reader next: %w", err)
		return
	}

	rc = &readCloser{tarReader, gzipReader}
	return

}
