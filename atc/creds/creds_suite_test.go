package creds_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLockrunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Creds Suite")
}
