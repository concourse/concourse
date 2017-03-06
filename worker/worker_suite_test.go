package worker_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWorker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Suite")
}

func fakeDBNGResourceType(t atc.ResourceType) *dbngfakes.FakeResourceType {
	fake := new(dbngfakes.FakeResourceType)
	fake.NameReturns(t.Name)
	fake.TypeReturns(t.Type)
	fake.SourceReturns(t.Source)
	return fake
}
