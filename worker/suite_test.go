package worker_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestWorker(t *testing.T) {
	SetDefaultEventuallyTimeout(time.Minute)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Suite")
}
