package gitlab_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGitlab(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gitlab Suite")
}
