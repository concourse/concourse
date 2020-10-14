package dbtest

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/gomega"
)

// Scenario represents the state of the world for testing.
type Scenario struct {
	Team     db.Team
	Pipeline db.Pipeline
	Workers  []db.Worker
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
