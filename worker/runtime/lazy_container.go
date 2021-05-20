package runtime

import (
	"context"
	"io"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/worker/runtime/libcontainerd"
)

type LazyContainer struct {
	ID string

	client        libcontainerd.Client
	killer        Killer
	rootfsManager RootfsManager

	container *Container
}

func (c *LazyContainer) ensureContainer() error {
	if c.container != nil {
		return nil
	}

	ctx := context.Background()
	container, err := c.client.GetContainer(ctx, c.ID)
	if err != nil {
		return err
	}

	c.container = NewContainer(container, c.killer, c.rootfsManager)
	return nil
}

var _ garden.Container = (*LazyContainer)(nil)

func (c *LazyContainer) Handle() string {
	return c.ID
}

func (c *LazyContainer) Stop(kill bool) error {
	if err := c.ensureContainer(); err != nil {
		return err
	}
	return c.container.Stop(kill)
}

func (c *LazyContainer) Run(spec garden.ProcessSpec, processIO garden.ProcessIO) (garden.Process, error) {
	if err := c.ensureContainer(); err != nil {
		return nil, err
	}
	return c.container.Run(spec, processIO)
}

func (c *LazyContainer) Attach(pid string, processIO garden.ProcessIO) (garden.Process, error) {
	if err := c.ensureContainer(); err != nil {
		return nil, err
	}
	return c.container.Attach(pid, processIO)
}

func (c *LazyContainer) Properties() (garden.Properties, error) {
	if err := c.ensureContainer(); err != nil {
		return garden.Properties{}, err
	}
	return c.container.Properties()
}

func (c *LazyContainer) Property(name string) (string, error) {
	if err := c.ensureContainer(); err != nil {
		return "", err
	}
	return c.container.Property(name)
}

func (c *LazyContainer) SetProperty(name string, value string) error {
	if err := c.ensureContainer(); err != nil {
		return err
	}
	return c.container.SetProperty(name, value)
}

func (c *LazyContainer) RemoveProperty(name string) error {
	if err := c.ensureContainer(); err != nil {
		return err
	}
	return c.container.RemoveProperty(name)
}

func (c *LazyContainer) Info() (garden.ContainerInfo, error) {
	if err := c.ensureContainer(); err != nil {
		return garden.ContainerInfo{}, err
	}
	return c.container.Info()
}

func (c *LazyContainer) Metrics() (garden.Metrics, error) {
	if err := c.ensureContainer(); err != nil {
		return garden.Metrics{}, err
	}
	return c.container.Metrics()
}

func (c *LazyContainer) StreamIn(spec garden.StreamInSpec) error {
	if err := c.ensureContainer(); err != nil {
		return err
	}
	return c.container.StreamIn(spec)
}

func (c *LazyContainer) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	if err := c.ensureContainer(); err != nil {
		return nil, err
	}
	return c.container.StreamOut(spec)
}

func (c *LazyContainer) SetGraceTime(graceTime time.Duration) error {
	if err := c.ensureContainer(); err != nil {
		return err
	}
	return c.container.SetGraceTime(graceTime)
}

func (c *LazyContainer) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	if err := c.ensureContainer(); err != nil {
		return garden.BandwidthLimits{}, err
	}
	return c.container.CurrentBandwidthLimits()
}

func (c *LazyContainer) CurrentCPULimits() (garden.CPULimits, error) {
	if err := c.ensureContainer(); err != nil {
		return garden.CPULimits{}, err
	}
	return c.container.CurrentCPULimits()
}

func (c *LazyContainer) CurrentDiskLimits() (garden.DiskLimits, error) {
	if err := c.ensureContainer(); err != nil {
		return garden.DiskLimits{}, err
	}
	return c.container.CurrentDiskLimits()
}

func (c *LazyContainer) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	if err := c.ensureContainer(); err != nil {
		return garden.MemoryLimits{}, err
	}
	return c.container.CurrentMemoryLimits()
}

func (c *LazyContainer) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	if err := c.ensureContainer(); err != nil {
		return 0, 0, err
	}
	return c.container.NetIn(hostPort, containerPort)
}

func (c *LazyContainer) NetOut(netOutRule garden.NetOutRule) error {
	if err := c.ensureContainer(); err != nil {
		return err
	}
	return c.container.NetOut(netOutRule)
}

func (c *LazyContainer) BulkNetOut(netOutRules []garden.NetOutRule) error {
	if err := c.ensureContainer(); err != nil {
		return err
	}
	return c.container.BulkNetOut(netOutRules)
}
