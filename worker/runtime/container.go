package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const GraceTimeKey = "garden.grace-time"

type UserNotFoundError struct {
	User string
}

func (u UserNotFoundError) Error() string {
	return fmt.Sprintf("user '%s' not found: no matching entries in /etc/passwd", u.User)
}

type Container struct {
	container     containerd.Container
	killer        Killer
	rootfsManager RootfsManager
}

func NewContainer(
	container containerd.Container,
	killer Killer,
	rootfsManager RootfsManager,
) *Container {
	return &Container{
		container:     container,
		killer:        killer,
		rootfsManager: rootfsManager,
	}
}

var _ garden.Container = (*Container)(nil)

func (c *Container) Handle() string {
	return c.container.ID()
}

// Stop stops a container.
//
func (c *Container) Stop(kill bool) error {
	ctx := context.Background()

	task, err := c.container.Task(ctx, cio.Load)
	if err != nil {
		return fmt.Errorf("task lookup: %w", err)
	}

	behaviour := KillGracefully
	if kill {
		behaviour = KillUngracefully
	}

	err = c.killer.Kill(ctx, task, behaviour)
	if err != nil {
		return fmt.Errorf("kill: %w", err)
	}

	return nil
}

// Run a process inside the container.
//
func (c *Container) Run(
	spec garden.ProcessSpec,
	processIO garden.ProcessIO,
) (garden.Process, error) {
	ctx := context.Background()

	containerSpec, err := c.container.Spec(ctx)
	if err != nil {
		return nil, fmt.Errorf("container spec: %w", err)
	}

	procSpec, err := c.setupContainerdProcSpec(spec, *containerSpec)
	if err != nil {
		return nil, err
	}

	err = c.rootfsManager.SetupCwd(containerSpec.Root.Path, procSpec.Cwd)
	if err != nil {
		return nil, fmt.Errorf("setup cwd: %w", err)
	}

	task, err := c.container.Task(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("task retrieval: %w", err)
	}

	id := procID(spec)
	cioOpts := containerdCIO(processIO, spec.TTY != nil)

	proc, err := task.Exec(ctx, id, &procSpec, cio.NewCreator(cioOpts...))
	if err != nil {
		return nil, fmt.Errorf("task exec: %w", err)
	}

	exitStatusC, err := proc.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("proc wait: %w", err)
	}

	err = proc.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("proc start: %w", err)
	}

	err = proc.CloseIO(ctx, containerd.WithStdinCloser)
	if err != nil {
		return nil, fmt.Errorf("proc closeio: %w", err)
	}

	return NewProcess(proc, exitStatusC), nil
}

// Attach starts streaming the output back to the client from a specified process.
//
func (c *Container) Attach(pid string, processIO garden.ProcessIO) (process garden.Process, err error) {
	ctx := context.Background()

	if pid == "" {
		return nil, ErrInvalidInput("empty pid")
	}

	task, err := c.container.Task(ctx, cio.Load)
	if err != nil {
		return nil, fmt.Errorf("task: %w", err)
	}

	cioOpts := []cio.Opt{
		cio.WithStreams(
			processIO.Stdin,
			processIO.Stdout,
			processIO.Stderr,
		),
	}

	proc, err := task.LoadProcess(ctx, pid, cio.NewAttach(cioOpts...))
	if err != nil {
		return nil, fmt.Errorf("load proc: %w", err)
	}

	status, err := proc.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("proc status: %w", err)
	}

	if status.Status != containerd.Running {
		return nil, fmt.Errorf("proc not running: status = %s", status.Status)
	}

	exitStatusC, err := proc.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("proc wait: %w", err)
	}

	return NewProcess(proc, exitStatusC), nil
}

// Properties returns the current set of properties
//
func (c *Container) Properties() (garden.Properties, error) {
	ctx := context.Background()

	labels, err := c.container.Labels(ctx)
	if err != nil {
		return garden.Properties{}, fmt.Errorf("labels retrieval: %w", err)
	}

	return labels, nil
}

// Property returns the value of the property with the specified name.
//
func (c *Container) Property(name string) (string, error) {
	properties, err := c.Properties()
	if err != nil {
		return "", err
	}

	v, found := properties[name]
	if !found {
		return "", ErrNotFound(name)
	}

	return v, nil
}

// Set a named property on a container to a specified value.
//
func (c *Container) SetProperty(name string, value string) error {
	labelSet := map[string]string{
		name: value,
	}

	_, err := c.container.SetLabels(context.Background(), labelSet)
	if err != nil {
		return fmt.Errorf("set label: %w", err)
	}

	return nil
}

// RemoveProperty - Not Implemented
func (c *Container) RemoveProperty(name string) (err error) {
	err = ErrNotImplemented
	return
}

// Info - Not Implemented
func (c *Container) Info() (info garden.ContainerInfo, err error) {
	err = ErrNotImplemented
	return
}

// Metrics - Not Implemented
func (c *Container) Metrics() (metrics garden.Metrics, err error) {
	err = ErrNotImplemented
	return
}

// StreamIn - Not Implemented
func (c *Container) StreamIn(spec garden.StreamInSpec) (err error) {
	err = ErrNotImplemented
	return
}

// StreamOut - Not Implemented
func (c *Container) StreamOut(spec garden.StreamOutSpec) (readCloser io.ReadCloser, err error) {
	err = ErrNotImplemented
	return
}

// SetGraceTime stores the grace time as a containerd label with key "garden.grace-time"
//
func (c *Container) SetGraceTime(graceTime time.Duration) error {
	err := c.SetProperty(GraceTimeKey, fmt.Sprintf("%d", graceTime))
	if err != nil {
		return fmt.Errorf("set grace time: %w", err)
	}

	return nil
}

// CurrentBandwidthLimits returns no limits (achieves parity with Guardian)
func (c *Container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return garden.BandwidthLimits{}, nil
}

// CurrentCPULimits returns the CPU shares allocated to the container
func (c *Container) CurrentCPULimits() (garden.CPULimits, error) {
	spec, err := c.container.Spec(context.Background())
	if err != nil {
		return garden.CPULimits{}, err
	}

	if spec == nil ||
		spec.Linux == nil ||
		spec.Linux.Resources == nil ||
		spec.Linux.Resources.CPU == nil ||
		spec.Linux.Resources.CPU.Shares == nil {
		return garden.CPULimits{}, nil
	}

	return garden.CPULimits{
		Weight: *spec.Linux.Resources.CPU.Shares,
	}, nil
}

// CurrentDiskLimits returns no limits (achieves parity with Guardian)
func (c *Container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return garden.DiskLimits{}, nil
}

// CurrentMemoryLimits returns the memory limit in bytes allocated to the container
func (c *Container) CurrentMemoryLimits() (limits garden.MemoryLimits, err error) {
	spec, err := c.container.Spec(context.Background())
	if err != nil {
		return garden.MemoryLimits{}, err
	}

	if spec == nil ||
		spec.Linux == nil ||
		spec.Linux.Resources == nil ||
		spec.Linux.Resources.Memory == nil ||
		spec.Linux.Resources.Memory.Limit == nil {
		return garden.MemoryLimits{}, nil
	}

	return garden.MemoryLimits{
		LimitInBytes: uint64(*spec.Linux.Resources.Memory.Limit),
	}, nil
}

// NetIn - Not Implemented
func (c *Container) NetIn(hostPort, containerPort uint32) (a, b uint32, err error) {
	err = ErrNotImplemented
	return
}

// NetOut - Not Implemented
func (c *Container) NetOut(netOutRule garden.NetOutRule) (err error) {
	err = ErrNotImplemented
	return
}

// BulkNetOut - Not Implemented
func (c *Container) BulkNetOut(netOutRules []garden.NetOutRule) (err error) {
	err = ErrNotImplemented
	return
}

func procID(gdnProcSpec garden.ProcessSpec) string {
	id := gdnProcSpec.ID
	if id == "" {
		uuid, err := uuid.NewV4()
		if err != nil {
			panic(fmt.Errorf("uuid gen: %w", err))
		}

		id = uuid.String()
	}

	return id
}

func (c *Container) setupContainerdProcSpec(gdnProcSpec garden.ProcessSpec, containerSpec specs.Spec) (specs.Process, error) {
	procSpec := containerSpec.Process

	procSpec.Args = append([]string{gdnProcSpec.Path}, gdnProcSpec.Args...)
	procSpec.Env = append(procSpec.Env, gdnProcSpec.Env...)

	cwd := gdnProcSpec.Dir
	if cwd == "" {
		cwd = "/"
	}

	procSpec.Cwd = cwd

	if gdnProcSpec.TTY != nil {
		procSpec.Terminal = true

		if gdnProcSpec.TTY.WindowSize != nil {
			procSpec.ConsoleSize = &specs.Box{
				Width:  uint(gdnProcSpec.TTY.WindowSize.Columns),
				Height: uint(gdnProcSpec.TTY.WindowSize.Rows),
			}
		}
	}


	if gdnProcSpec.User != "" {
		var ok bool
		var err error
		procSpec.User, ok, err = c.rootfsManager.LookupUser(containerSpec.Root.Path, gdnProcSpec.User)
		if err != nil {
			return specs.Process{}, fmt.Errorf("lookup user: %w", err)
		}
		if !ok {
			return specs.Process{}, UserNotFoundError{User: gdnProcSpec.User}
		}

		setUserEnv := fmt.Sprintf("USER=%s", gdnProcSpec.User)
		procSpec.Env = append(procSpec.Env, setUserEnv)
	}
	return *procSpec, nil
}

func containerdCIO(gdnProcIO garden.ProcessIO, tty bool) []cio.Opt {
	cioOpts := []cio.Opt{
		cio.WithStreams(
			gdnProcIO.Stdin,
			gdnProcIO.Stdout,
			gdnProcIO.Stderr,
		),
	}

	if tty {
		cioOpts = append(cioOpts, cio.WithTerminal)
	}

	return cioOpts
}
