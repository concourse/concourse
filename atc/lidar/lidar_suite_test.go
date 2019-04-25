package lidar_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLidar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lidar Suite")
}
