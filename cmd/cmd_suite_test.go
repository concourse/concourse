package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
}
