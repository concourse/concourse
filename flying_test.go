package testflight_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Flying", func() {
	var tmpdir string

	BeforeEach(func() {
		var err error

		tmpdir, err = ioutil.TempDir("", "fly-test")
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(tmpdir, "build.yml"), []byte(fmt.Sprintf(`---
---
image: %s

params:
  FOO: 1

run:
  path: bash
  args:
    - -c
    - |
      echo some output
      echo FOO is $FOO
      exit 0
`, helperRootfs)), 0644)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	It("works", func() {
		fly := exec.Command(builtComponents["fly"])
		fly.Dir = tmpdir

		session, err := gexec.Start(fly, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, 10*time.Minute).Should(gexec.Exit(0))

		Ω(session).Should(gbytes.Say("some output"))
		Ω(session).Should(gbytes.Say("FOO is 1"))
	})
})
