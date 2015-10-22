package ui_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestUi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ui Suite")
}
