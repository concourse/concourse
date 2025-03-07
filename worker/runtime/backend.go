//go:build linux

// Package backend provides the implementation of a Garden server backed by
// containerd.
//
// See https://containerd.io/, and https://github.com/cloudfoundry/garden.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/runtime/libcontainerd"
	bespec "github.com/concourse/concourse/worker/runtime/spec"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

var _ garden.Backend = (*GardenBackend)(nil)

// GardenBackend implements a Garden backend backed by `containerd`.
type GardenBackend struct {
	client        libcontainerd.Client
	killer        Killer
	network       Network
	rootfsManager RootfsManager
	userNamespace UserNamespace
	initBinPath   string
	// override path for the seccomp profile
	seccompProfilePath string
	// content of the upper path, or the default ones (default or privileged)
	seccompProfile     specs.LinuxSeccomp
	seccompProfileFuse specs.LinuxSeccomp
	// path to the hosts OCI hooks dir
	ociHooksDir string
	// the deserialized hooks
	ociHooks specs.Hooks

	maxContainers  int
	requestTimeout time.Duration
	createLock     TimeoutWithByPassLock
	privilegedMode bespec.PrivilegedMode
}

//counterfeiter:generate . UserNamespace
type UserNamespace interface {
	MaxValidIds() (uid, gid uint32, err error)
}

func WithUserNamespace(s UserNamespace) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.userNamespace = s
	}
}

// GardenBackendOpt defines a functional option that when applied, modifies the
// configuration of a GardenBackend.
type GardenBackendOpt func(b *GardenBackend)

// WithRootfsManager configures the RootfsManager used by the backend.
func WithRootfsManager(r RootfsManager) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.rootfsManager = r
	}
}

// WithKiller configures the killer used to terminate tasks.
func WithKiller(k Killer) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.killer = k
	}
}

// WithNetwork configures the network used by the backend.
func WithNetwork(n Network) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.network = n
	}
}

// WithMaxContainers configures the max number of containers that can be created
func WithMaxContainers(limit int) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.maxContainers = limit
	}
}

// WithRequestTimeout configures the request timeout
// Currently only used as timeout for acquiring the create container lock
func WithRequestTimeout(requestTimeout time.Duration) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.requestTimeout = requestTimeout
	}
}

// WithInitBinPath configures the path to the init binary that is injected into every container.
// The init binary just sits there doing nothing until Concourse decides it's time to attach to the container
// and exec the actual command
func WithInitBinPath(initBinPath string) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.initBinPath = initBinPath
	}
}

func WithSeccompProfilePath(seccompProfilePath string) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.seccompProfilePath = seccompProfilePath
	}
}

func WithOciHooksDir(ociHooksDir string) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.ociHooksDir = ociHooksDir
	}
}

func WithPrivilegedMode(privilegedMode bespec.PrivilegedMode) GardenBackendOpt {
	return func(b *GardenBackend) {
		b.privilegedMode = privilegedMode
	}
}

type When struct {
	Always      bool              `json:"always,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Commands    []string          `json:"commands,omitempty"`
}

// Hook specifies a command that is run at a particular event in the lifecycle of a container
type HookFile struct {
	Version string     `json:"version"`
	Hook    specs.Hook `json:"hook"`
	When    When       `json:"when,omitempty"`
	Stages  []string   `json:"stages,omitempty"`
}

// NewGardenBackend instantiates a GardenBackend with tweakable configurations passed as Config.
func NewGardenBackend(client libcontainerd.Client, opts ...GardenBackendOpt) (b GardenBackend, err error) {
	if client == nil {
		err = ErrInvalidInput("nil client")
		return
	}

	b = GardenBackend{
		client: client,
	}
	for _, opt := range opts {
		opt(&b)
	}

	var enableLock bool
	if b.maxContainers != 0 {
		enableLock = true
	}
	b.createLock = NewTimeoutLimitLock(b.requestTimeout, enableLock)

	if b.network == nil {
		b.network, err = NewCNINetwork()
		if err != nil {
			return b, fmt.Errorf("network init: %w", err)
		}
	}

	var hooks specs.Hooks
	if b.ociHooksDir != "" {
		files, err := os.ReadDir(b.ociHooksDir)
		if err != nil {
			return b, fmt.Errorf("ociHooksDir: %w", err)
		}

		for _, direntry := range files {
			if direntry.IsDir() {
				continue
			}
			var f = b.ociHooksDir + "/" + direntry.Name()
			var hookJsonContent, err = os.ReadFile(f)
			if err != nil {
				return b, fmt.Errorf("ociHooksDir file: %w", err)
			}
			fmt.Println("Parsing hooks file", f)
			var hooksParsed HookFile
			var err2 = json.Unmarshal(hookJsonContent, &hooksParsed)
			if err2 != nil {
				return b, fmt.Errorf("ociHooks file failed to parse: %w", err2)
			}
			for _, stage := range hooksParsed.Stages {
				fmt.Println("Add hook to stage", stage)
				switch stage {
				case "prestart":
					hooks.Prestart = append(hooks.Prestart, hooksParsed.Hook)
				case "createRuntime":
					hooks.CreateRuntime = append(hooks.CreateRuntime, hooksParsed.Hook)
				case "createContainer":
					hooks.CreateContainer = append(hooks.CreateContainer, hooksParsed.Hook)
				case "startContainer":
					hooks.StartContainer = append(hooks.StartContainer, hooksParsed.Hook)
				case "poststart":
					hooks.Poststart = append(hooks.Poststart, hooksParsed.Hook)
				case "poststop":
					hooks.Poststop = append(hooks.Poststop, hooksParsed.Hook)
				}
			}
		}
	}
	b.ociHooks = hooks

	if b.seccompProfilePath != "" {
		var seccompJsonContent, err = os.ReadFile(b.seccompProfilePath)
		if err != nil {
			return b, fmt.Errorf("seccomp file: %w", err)
		}
		var profile specs.LinuxSeccomp
		var err2 = json.Unmarshal(seccompJsonContent, &profile)
		if err2 != nil {
			return b, fmt.Errorf("seccomp file failed to parse: %w", err2)
		}
		b.seccompProfile = profile
		b.seccompProfileFuse = profile
	} else {
		b.seccompProfile = bespec.GetDefaultSeccompProfile()
		b.seccompProfileFuse = bespec.GetDefaultSeccompProfileFuse()
	}

	if b.killer == nil {
		b.killer = NewKiller()
	}

	if b.rootfsManager == nil {
		b.rootfsManager = NewRootfsManager()
	}

	if b.userNamespace == nil {
		b.userNamespace = NewUserNamespace()
	}

	// Because the garden server is created programmatically in the integration tests, add
	// a sane default path
	if b.initBinPath == "" {
		b.initBinPath = bespec.DefaultInitBinPath
	}

	return b, nil
}

// Start initializes the client.
func (b *GardenBackend) Start() (err error) {
	err = b.client.Init()
	if err != nil {
		return fmt.Errorf("client init: %w", err)
	}

	err = b.network.SetupHostNetwork()
	if err != nil {
		return fmt.Errorf("setup host network failed: %w", err)
	}

	return
}

// Stop closes the client's underlying connections and frees any resources
// associated with it.
func (b *GardenBackend) Stop() (err error) {
	err = b.client.Stop()
	if err != nil {
		return fmt.Errorf("client stop: %w", err)
	}

	return
}

// Ping pings the garden server in order to check connectivity.
func (b *GardenBackend) Ping() (err error) {
	err = b.client.Version(context.Background())
	if err != nil {
		return fmt.Errorf("getting containerd version: %w", err)
	}

	return
}

// Create creates a new container.
func (b *GardenBackend) Create(gdnSpec garden.ContainerSpec) (garden.Container, error) {
	ctx := context.Background()

	cont, err := b.createContainer(ctx, gdnSpec)
	if err != nil {
		return nil, fmt.Errorf("new container: %w", err)
	}

	err = b.startTask(ctx, cont, b.isHermetic(gdnSpec))
	if err != nil {
		return nil, fmt.Errorf("starting task: %w", err)
	}

	return NewContainer(
		cont,
		b.killer,
		b.rootfsManager,
	), nil
}

func (b *GardenBackend) isHermetic(gdnSpec garden.ContainerSpec) bool {
	if len(gdnSpec.NetOut) != 0 {
		return false
	}

	return true
}

func (b *GardenBackend) createContainer(ctx context.Context, gdnSpec garden.ContainerSpec) (containerd.Container, error) {
	err := b.createLock.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring create container lock: %w", err)

	}
	defer b.createLock.Release()

	err = b.checkContainerCapacity(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking container capacity: %w", err)
	}

	maxUid, maxGid, err := b.userNamespace.MaxValidIds()
	if err != nil {
		return nil, fmt.Errorf("getting uid and gid maps: %w", err)
	}

	oci, err := bespec.OciSpec(b.initBinPath, b.seccompProfile, b.seccompProfileFuse, b.ociHooks, b.privilegedMode, gdnSpec, maxUid, maxGid)
	if err != nil {
		return nil, fmt.Errorf("garden spec to oci spec: %w", err)
	}

	netMounts, err := b.network.SetupMounts(gdnSpec.Handle)
	if err != nil {
		return nil, fmt.Errorf("network setup mounts: %w", err)
	}

	oci.Mounts = append(oci.Mounts, netMounts...)

	labels, err := propertiesToLabels(gdnSpec.Properties)
	if err != nil {
		return nil, fmt.Errorf("convert properties to labels: %w", err)
	}
	return b.client.NewContainer(ctx, gdnSpec.Handle, labels, oci)
}

func (b *GardenBackend) startTask(ctx context.Context, cont containerd.Container, hermetic bool) error {
	task, err := cont.NewTask(ctx, cio.NullIO, containerd.WithNoNewKeyring)
	if err != nil {
		return fmt.Errorf("new task: %w", err)
	}

	err = b.network.Add(ctx, task, cont.ID())
	if err != nil {
		return fmt.Errorf("network add: %w", err)
	}

	if hermetic {
		err = b.network.DropContainerTraffic(cont.ID())
		if err != nil {
			return fmt.Errorf("network drop container traffic: %w", err)
		}
	}

	return task.Start(ctx)
}

// Destroy gracefully destroys a container.
func (b *GardenBackend) Destroy(handle string) error {
	if handle == "" {
		return ErrInvalidInput("empty handle")
	}

	ctx := context.Background()

	container, err := b.client.GetContainer(ctx, handle)
	if err != nil {
		return fmt.Errorf("get container: %w", err)
	}

	task, err := container.Task(ctx, cio.Load)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("task lookup: %w", err)
		}

		err = container.Delete(ctx)
		if err != nil {
			return fmt.Errorf("deleting container: %w", err)
		}

		return nil
	}

	err = b.killer.Kill(ctx, task, KillGracefully)
	if err != nil {
		return fmt.Errorf("gracefully killing task: %w", err)
	}

	err = b.network.ResumeContainerTraffic(handle)
	if err != nil {
		if !errors.Is(err, ErrGettingContainerIP) {
			return fmt.Errorf("resume container traffic: %w", err)
		}
	}

	err = b.network.Remove(ctx, task, handle)
	if err != nil {
		return fmt.Errorf("network remove: %w", err)
	}

	_, err = task.Delete(ctx, containerd.WithProcessKill)
	if err != nil {
		return fmt.Errorf("task remove: %w", err)
	}

	err = container.Delete(ctx)
	if err != nil {
		return fmt.Errorf("deleting container: %w", err)
	}

	return nil
}

// Containers lists all containers filtered by properties (which are ANDed
// together).
func (b *GardenBackend) Containers(properties garden.Properties) ([]garden.Container, error) {
	filters, err := propertiesToFilterList(properties)
	if err != nil {
		return nil, err
	}

	res, err := b.client.Containers(context.Background(), filters...)
	if err != nil {
		err = fmt.Errorf("list containers: %w", err)
		return nil, err
	}

	containers := make([]garden.Container, len(res))
	for i, containerdContainer := range res {
		containers[i] = NewContainer(
			containerdContainer,
			b.killer,
			b.rootfsManager,
		)
	}

	return containers, nil
}

// Lookup returns the container with the specified handle.
func (b *GardenBackend) Lookup(handle string) (garden.Container, error) {
	if handle == "" {
		return nil, ErrInvalidInput("empty handle")
	}

	containerdContainer, err := b.client.GetContainer(context.Background(), handle)
	if err != nil {
		return nil, fmt.Errorf("get container: %w", err)
	}

	return NewContainer(
		containerdContainer,
		b.killer,
		b.rootfsManager,
	), nil
}

// GraceTime returns the value of the "garden.grace-time" property
func (b *GardenBackend) GraceTime(container garden.Container) (duration time.Duration) {
	property, err := container.Property(GraceTimeKey)
	if err != nil {
		return 0
	}

	_, err = fmt.Sscanf(property, "%d", &duration)
	if err != nil {
		return 0
	}

	return duration
}

// Capacity - Not Implemented
func (b *GardenBackend) Capacity() (capacity garden.Capacity, err error) {
	err = ErrNotImplemented
	return
}

// BulkInfo - Not Implemented
func (b *GardenBackend) BulkInfo(handles []string) (info map[string]garden.ContainerInfoEntry, err error) {
	err = ErrNotImplemented
	return
}

// BulkMetrics - Not Implemented
func (b *GardenBackend) BulkMetrics(handles []string) (metrics map[string]garden.ContainerMetricsEntry, err error) {
	err = ErrNotImplemented
	return
}

// checkContainerCapacity ensures that Garden.MaxContainers is respected
func (b *GardenBackend) checkContainerCapacity(ctx context.Context) error {
	if b.maxContainers == 0 {
		return nil
	}

	containers, err := b.client.Containers(ctx)
	if err != nil {
		return fmt.Errorf("getting list of containers: %w", err)
	}

	if len(containers) >= b.maxContainers {
		return fmt.Errorf("max containers reached")
	}
	return nil
}
