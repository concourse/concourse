package gardenruntimetest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc/worker/gclient"
)

type Garden struct {
	ContainerList []*Container
}

func (g Garden) FindContainer(handle string) (*Container, int, bool) {
	for i, c := range g.ContainerList {
		if c.handle == handle {
			return c, i, true
		}
	}
	return nil, 0, false
}

func (g Garden) FilteredContainers(pred func(*Container) bool) []*Container {
	var filtered []*Container
	for _, c := range g.ContainerList {
		if pred(c) {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func (g Garden) Ping() (err error)                               { return }
func (g Garden) Capacity() (capacity garden.Capacity, err error) { return }

func (g *Garden) Create(spec garden.ContainerSpec) (gclient.Container, error) {
	handle := spec.Handle
	if handle == "" {
		return nil, errors.New("spec.Handle should be set")
	}
	if handle == "fail-to-create" {
		return nil, errors.New("failed to create (because handle is fail-to-create)")
	}
	if _, _, ok := g.FindContainer(handle); ok {
		return nil, fmt.Errorf("handle %s already exists", handle)
	}
	container := NewContainer(handle).WithSpec(spec)
	g.ContainerList = append(g.ContainerList, container)
	return container, nil
}

func (g *Garden) Destroy(handle string) error {
	g.ContainerList = g.FilteredContainers(func(c *Container) bool {
		return c.handle != handle
	})
	return nil
}

func (g *Garden) Containers(filter garden.Properties) ([]gclient.Container, error) {
	filteredContainers := g.FilteredContainers(func(c *Container) bool {
		return matchesFilter(c.Spec.Properties, filter)
	})
	containers := make([]gclient.Container, len(filteredContainers))
	for i, c := range filteredContainers {
		containers[i] = c
	}
	return containers, nil
}

func (g *Garden) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	panic("not implemented")
}

func (g *Garden) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	panic("not implemented")
}

func (g *Garden) Lookup(handle string) (gclient.Container, error) {
	c, _, ok := g.FindContainer(handle)
	if !ok {
		return nil, garden.ContainerNotFoundError{Handle: handle}
	}
	return c, nil
}

func NewContainer(handle string) *Container {
	return &Container{handle: handle}
}

type Container struct {
	handle string
	Spec   garden.ContainerSpec

	processMtx sync.Mutex
	Processes  []*Process
}

func (c Container) WithSpec(spec garden.ContainerSpec) *Container {
	c.Spec = spec
	return &c
}

func (c Container) WithProcesses(processes ...*Process) *Container {
	newProcesses := make([]*Process, len(c.Processes)+len(processes))
	copy(newProcesses, c.Processes)
	copy(newProcesses[len(c.Processes):], processes)
	return &c
}

func (c Container) Handle() string {
	return c.handle
}

func (c Container) Stop(kill bool) error {
	sig := garden.SignalTerminate
	if kill {
		sig = garden.SignalKill
	}
	for _, proc := range c.Processes {
		proc.Signal(sig)
	}
	return nil
}

func (c Container) Info() (garden.ContainerInfo, error)      { panic("not implemented") }
func (c *Container) StreamIn(spec garden.StreamInSpec) error { panic("not implemented") }
func (c *Container) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	panic("not implemented")
}
func (c *Container) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return c.Spec.Limits.Bandwidth, nil
}

func (c *Container) CurrentCPULimits() (garden.CPULimits, error) {
	return c.Spec.Limits.CPU, nil
}

func (c *Container) CurrentDiskLimits() (garden.DiskLimits, error) {
	return c.Spec.Limits.Disk, nil
}

func (c *Container) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return c.Spec.Limits.Memory, nil
}

func (c *Container) NetIn(hostPort uint32, containerPort uint32) (uint32, uint32, error) {
	panic("not implemented")
}

func (c *Container) NetOut(netOutRule garden.NetOutRule) error {
	panic("not implemented")
}

func (c *Container) BulkNetOut(netOutRules []garden.NetOutRule) error {
	panic("not implemented")
}

func (c *Container) Run(ctx context.Context, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	c.processMtx.Lock()
	defer c.processMtx.Unlock()

	if spec.ID == "" {
		return nil, errors.New("spec.ID should be set (deterministically), but was unset")
	}

	if spec.Path == "exe-not-found" {
		return nil, garden.ExecutableNotFoundError{Message: "exe not found (because Path was 'exe-not-found')"}
	}

	proc := NewProcess(spec.ID, spec)
	proc.AddIO(io)

	go proc.Run()

	c.Processes = append(c.Processes, proc)
	return proc, nil
}

func (c *Container) Attach(ctx context.Context, processID string, io garden.ProcessIO) (garden.Process, error) {
	c.processMtx.Lock()
	defer c.processMtx.Unlock()

	for _, proc := range c.Processes {
		if proc.id == processID {
			proc.AddIO(io)
			return proc, nil
		}
	}
	return nil, errors.New("process not found")
}

func (c *Container) NumProcesses() int {
	c.processMtx.Lock()
	defer c.processMtx.Unlock()

	return len(c.Processes)
}

func (c *Container) Metrics() (garden.Metrics, error) { panic("not implemented") }

func (c *Container) SetGraceTime(graceTime time.Duration) error {
	return nil
}

func (c *Container) properties() garden.Properties {
	if c.Spec.Properties == nil {
		c.Spec.Properties = make(garden.Properties)
	}
	return c.Spec.Properties
}

func (c *Container) Properties() (garden.Properties, error) {
	return c.properties(), nil
}

func (c *Container) Property(name string) (string, error) {
	v, ok := c.properties()[name]
	if !ok {
		return "", errors.New("missing property " + name)
	}
	return v, nil
}

func (c *Container) SetProperty(name string, value string) error {
	c.properties()[name] = value
	return nil
}

func (c *Container) RemoveProperty(name string) error {
	delete(c.properties(), name)
	return nil
}

type Process struct {
	id         string
	Spec       garden.ProcessSpec
	StopSignal *garden.Signal

	ioMtx sync.Mutex
	io    []garden.ProcessIO

	wait     chan struct{}
	exitCode int
}

func NewProcess(id string, spec garden.ProcessSpec) *Process {
	return &Process{
		id:   id,
		Spec: spec,

		wait: make(chan struct{}),
	}
}

func (p *Process) AddIO(io garden.ProcessIO) {
	p.ioMtx.Lock()
	defer p.ioMtx.Unlock()

	p.io = append(p.io, io)
}

func (p *Process) Run() {
	defer close(p.wait)

	switch p.Spec.Path {
	case "echo":
		p.WriteStdout([]byte(strings.Join(p.Spec.Args, " ") + "\n"))
		p.exitCode = 0
	case "sleep-and-echo":
		duration, err := time.ParseDuration(p.Spec.Args[0])
		if err != nil {
			panic(fmt.Sprintf("invalid duration: %v", err))
		}
		time.Sleep(duration)
		p.WriteStdout([]byte(strings.Join(p.Spec.Args[1:], " ") + "\n"))
		p.exitCode = 0
	case "noop":
		p.exitCode = 0
	default:
		panic(fmt.Sprintf("unsupported program %s", p.Spec.Path))
	}
}

func (p Process) ID() string { return p.id }
func (p *Process) Wait() (int, error) {
	<-p.wait
	return p.exitCode, nil
}
func (p *Process) WriteStdout(data []byte) {
	p.ioMtx.Lock()
	defer p.ioMtx.Unlock()

	for _, io := range p.io {
		io.Stdout.Write(data)
	}
}
func (p *Process) SetTTY(tty garden.TTYSpec) error {
	p.Spec.TTY = &tty
	return nil
}
func (p *Process) Signal(sig garden.Signal) error {
	p.StopSignal = &sig
	return nil
}
