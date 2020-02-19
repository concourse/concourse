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
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/kubernetes/backend"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Kubernetes struct {
	Namespace  string `long:"namespace"`
	InCluster  bool   `long:"in-cluster"`
	Kubeconfig string `long:"config" default:"~/.kube/config"`
	be         *backend.Backend
	wf         db.WorkerFactory
	rf         resource.ResourceFactory
}

func NewClient(
	inCluster bool,
	config, namespace string,
	dbWorkerFactory db.WorkerFactory,
	resourceFactory resource.ResourceFactory,
) (k Kubernetes, err error) {
	var cfg *rest.Config

	switch {
	case config != "":
		cfg, err = clientcmd.BuildConfigFromFlags("", config)
		if err != nil {
			return
		}
	case inCluster:
		cfg, err = rest.InClusterConfig()
		if err != nil {
			err = fmt.Errorf("incluster cfg: %w", err)
			return
		}
	default:
		err = fmt.Errorf("incluster or config must be specified")
		return
	}

	k.be, err = backend.New(namespace, cfg)
	if err != nil {
		err = fmt.Errorf("new backend: %w")
		return
	}

	k.wf = dbWorkerFactory
	k.rf = resourceFactory

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

// PodArtifact represents an artifact that can be retrieved from a pod.
//
type PodArtifact struct {
	Pod    string
	Ip     string
	Handle string
}

func (a PodArtifact) String() string {
	return strings.Join([]string{
		a.Pod,
		a.Handle,
		a.Ip,
	}, ":")
}

func UnmarshalPodArtifact(str string) (a PodArtifact) {
	parts := strings.SplitN(str, ":", 3)
	a.Pod, a.Handle, a.Ip = parts[0], parts[1], parts[2]
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

func (k Kubernetes) findOrCreateContainer(
	owner db.ContainerOwner,
	containerMetadata db.ContainerMetadata,
	containerSpec worker.ContainerSpec,
) (*backend.Container, error) {
	w, found, err := k.wf.GetWorker("k8s")
	if err != nil {
		return nil, fmt.Errorf("get worker: %w")
	}

	if !found {
		return nil, fmt.Errorf("no worker found")
	}

	creating, created, err := w.FindContainer(owner)
	if err != nil {
		return nil, fmt.Errorf("find container: %w")
	}

	var handle string

	switch {
	case creating != nil:
		handle = creating.Handle()
	case created != nil:
		handle = created.Handle()
	default:
		creating, err = w.CreateContainer(
			owner,
			containerMetadata,
		)
		if err != nil {
			return nil, fmt.Errorf("creating db container: %w", err)
		}

		handle = creating.Handle()
	}

	// TODO tell that it's a not found
	container, err := k.be.Lookup(handle)
	if err != nil {
		_, ok := err.(garden.ContainerNotFoundError)
		if !ok {
			return nil, fmt.Errorf("pod lookup: %w", err)
		}
	}

	if created != nil {
		if container == nil {
			// how come?
			return nil, fmt.Errorf("couldn't find pod of container marked as created: %s", handle)
		}
	}

	if container == nil {
		// figure the image out
		imageUri, err := k.fetchImageForContainer(containerSpec, w, creating)
		if err != nil {
			err = fmt.Errorf("fetch img for container: %w", err)

			_, transitionErr := creating.Failed() // TODO wrap this .. somehow?
			// would be nice to not miss this potential err
			// ps.: we should do this `ir err, FAILED + capture err`
			// everywhere here.

			if transitionErr != nil {
				err = multierror.Append(transitionErr, err)
			}

			return nil, err
		}

		inputs := make(map[string]string, len(containerSpec.ArtifactByPath))
		inputSources := make(map[string]string, len(containerSpec.ArtifactByPath))

		for dest, artifact := range containerSpec.ArtifactByPath {
			artf := UnmarshalPodArtifact(artifact.ID())
			uri := "http://" + artf.Ip + ":7788/volumes/" + artf.Handle + "/stream-out"
			name := filepath.Base(dest)

			inputs[name] = dest
			inputSources[uri] = dest
		}

		podDefinition := backend.Pod(
			backend.WithBase(handle),
			backend.WithBaggageclaim(), // could be noop if no outputs
			backend.WithInputsFetcher(inputSources,
				backend.WithInputs(inputs),
			),
			backend.WithMain(imageUri,
				backend.WithEnv(containerSpec.Env),
				backend.WithDir(containerSpec.Dir),
				backend.WithOutputs(containerSpec.Outputs),
				backend.WithInputs(inputs),
				backend.WithBaggageclaimVolumeMount(),
			),
		)

		// create the pod
		container, err = k.be.Create(handle, podDefinition)
		if err != nil {
			return nil, fmt.Errorf("creating container: %w", err)
		}

		created, err = creating.Created()
		if err != nil {
			return nil, fmt.Errorf("transitioning creating to created: %w", err)
		}
	}

	return container, nil
}
