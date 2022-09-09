package dbtest

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"

	"github.com/onsi/ginkgo/v2"
	//lint:ignore ST1001 this is used for tests
	. "github.com/onsi/gomega"
)

// Scenario represents the state of the world for testing.
type Scenario struct {
	Team     db.Team
	Pipeline db.Pipeline
	Workers  []db.Worker

	SpanContext db.SpanContext
}

type SetupFunc func(*Scenario) error

func Setup(setup ...SetupFunc) *Scenario {
	scenario := &Scenario{}
	scenario.Run(setup...)
	return scenario
}

func (scenario *Scenario) Run(setup ...SetupFunc) {
	for _, f := range setup {
		err := f(scenario)
		Expect(err).ToNot(HaveOccurred())
	}
}

func (scenario Scenario) Job(name string) db.Job {
	Expect(scenario.Pipeline).ToNot(BeNil(), "pipeline not set in scenario")

	job, found, err := scenario.Pipeline.Job(name)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "job '%s' not found", name)

	return job
}

func (scenario Scenario) Resource(name string) db.Resource {
	Expect(scenario.Pipeline).ToNot(BeNil(), "pipeline not set in scenario")

	resource, found, err := scenario.Pipeline.Resource(name)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "resource '%s' not found", name)

	return resource
}

func (scenario Scenario) ResourceType(name string) db.ResourceType {
	Expect(scenario.Pipeline).ToNot(BeNil(), "pipeline not set in scenario")

	resourceType, found, err := scenario.Pipeline.ResourceType(name)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "resource type '%s' not found", name)

	return resourceType
}

func (scenario Scenario) Prototype(name string) db.Prototype {
	Expect(scenario.Pipeline).ToNot(BeNil(), "pipeline not set in scenario")

	resourceType, found, err := scenario.Pipeline.Prototype(name)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "prototype '%s' not found", name)

	return resourceType
}

func (scenario Scenario) ResourceVersion(name string, version atc.Version) db.ResourceConfigVersion {
	Expect(scenario.Pipeline).ToNot(BeNil(), "pipeline not set in scenario")

	resource, found, err := scenario.Pipeline.Resource(name)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "resource '%s' not found", name)

	rcv, found, err := resource.FindVersion(version)
	Expect(err).ToNot(HaveOccurred())
	Expect(found).To(BeTrue(), "resource version '%v' of '%s' not found", version, name)

	return rcv
}

func (scenario Scenario) Worker(name string) db.Worker {
	for _, worker := range scenario.Workers {
		if worker.Name() == name {
			return worker
		}
	}

	ginkgo.Fail(fmt.Sprintf("worker '%s' not found", name))
	panic("unreachable")
}

func (scenario Scenario) Container(workerName string, owner db.ContainerOwner) db.Container {
	container, ok := scenario.FindContainer(workerName, owner)
	Expect(ok).To(BeTrue())
	return container
}

func (scenario Scenario) FindContainer(workerName string, owner db.ContainerOwner) (db.Container, bool) {
	worker := scenario.Worker(workerName)

	creating, created, err := worker.FindContainer(owner)
	Expect(err).ToNot(HaveOccurred())

	if created != nil {
		return created, true
	}
	if creating != nil {
		return creating, true
	}

	return nil, false
}
