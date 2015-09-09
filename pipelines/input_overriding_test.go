package git_pipeline_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/testflight/gitserver"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with multiple inputs", func() {
	var (
		gitServerA *gitserver.Server
		gitServerB *gitserver.Server

		firstGuidA string
		firstGuidB string

		tmpdir string
	)

	BeforeEach(func() {
		gitServerA = gitserver.Start(gitServerRootfs, gardenClient)
		gitServerB = gitserver.Start(gitServerRootfs, gardenClient)

		configurePipeline(
			"-c", "fixtures/many-inputs.yml",
			"-v", "git-server-a="+gitServerA.URI(),
			"-v", "git-server-b="+gitServerB.URI(),
		)

		firstGuidA = gitServerA.Commit()
		firstGuidB = gitServerB.Commit()

		var err error
		tmpdir, err = ioutil.TempDir("", "fly-test")
		Ω(err).ShouldNot(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(tmpdir, "task.yml"),
			[]byte(`---
platform: linux
inputs:
- name: git-repo-a
- name: git-repo-b
run:
  path: sh
  args: 
    - -c
    - |
      echo a has $(cat git-repo-a/guids)
      echo b has $(cat git-repo-b/guids)
`),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)

		gitServerA.Stop()
		gitServerB.Stop()
	})

	It("can have its inputs used as the basis for a one-off build", func() {
		watch := flyWatch("some-job")
		Ω(watch).Should(gbytes.Say("initializing"))
		Ω(watch).Should(gbytes.Say("a has " + firstGuidA))
		Ω(watch).Should(gbytes.Say("b has " + firstGuidB))
		Ω(watch).Should(gbytes.Say("succeeded"))
		Ω(watch).Should(gexec.Exit(0))

		fly := exec.Command(
			flyBin,
			"-t", atcURL,
			"execute",
			"-c", "task.yml",
			"--inputs-from-pipeline", pipelineName,
			"--inputs-from-job", "some-job",
		)
		fly.Dir = tmpdir

		execute := helpers.StartFly(fly)
		<-execute.Exited
		Ω(execute).Should(gbytes.Say("initializing"))
		Ω(execute).Should(gbytes.Say("a has " + firstGuidA))
		Ω(execute).Should(gbytes.Say("b has " + firstGuidB))
		Ω(execute).Should(gbytes.Say("succeeded"))
		Ω(execute).Should(gexec.Exit(0))

		err := ioutil.WriteFile(
			filepath.Join(tmpdir, "guids"),
			[]byte("some-overridden-guid"),
			0644,
		)
		Ω(err).ShouldNot(HaveOccurred())

		fly = exec.Command(
			flyBin,
			"-t", atcURL,
			"execute",
			"-c", "task.yml",
			"--inputs-from-pipeline", pipelineName,
			"--inputs-from-job", "some-job",
			"--input", "git-repo-b="+tmpdir,
		)
		fly.Dir = tmpdir

		execute = helpers.StartFly(fly)
		<-execute.Exited
		Ω(execute).Should(gbytes.Say("initializing"))
		Ω(execute).Should(gbytes.Say("a has " + firstGuidA))
		Ω(execute).Should(gbytes.Say("b has some-overridden-guid"))
		Ω(execute).Should(gbytes.Say("succeeded"))
		Ω(execute).Should(gexec.Exit(0))
	})
})
