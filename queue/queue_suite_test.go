package queue_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestQueue(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Queue Suite")
}
