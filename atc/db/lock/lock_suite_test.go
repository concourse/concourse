package lock_test

import (
	"github.com/concourse/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lock Suite")
}

var postgresRunner postgresrunner.Runner

var _ = postgresrunner.GinkgoRunner(&postgresRunner)
