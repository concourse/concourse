package flying_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/concourse/testflight/gitserver"
	"github.com/concourse/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Flying with an image_resource", func() {
	var (
		rootfsGitServer *gitserver.Server

		tmpdir  string
		fixture string
	)

	BeforeEach(func() {
		var err error

		rootfsGitServer = gitserver.Start(concourseClient)

		rootfsGitServer.CommitRootfs()

		tmpdir, err = ioutil.TempDir("", "fly-test")
		Expect(err).NotTo(HaveOccurred())

		fixture = filepath.Join(tmpdir, "fixture")

		err = os.MkdirAll(fixture, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(
			filepath.Join(fixture, "run"),
			[]byte(`#!/bin/sh
ls /bin
`),
			0755,
		)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		os.RemoveAll(tmpdir)
	})

	It("propagates the rootfs and metadata to the task", func() {
		err := ioutil.WriteFile(
			filepath.Join(fixture, "task.yml"),
			[]byte(`---
platform: linux

image_resource:
  type: git
  source: {uri: "`+rootfsGitServer.URI()+`"}

inputs:
- name: fixture

run:
  path: sh
  args:
  - -c
  - |
    ls /hello-im-a-git-rootfs
    echo $IMAGE_PROVIDED_ENV
`),
			0644,
		)
		Expect(err).NotTo(HaveOccurred())
		fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml")
		fly.Dir = fixture

		session := helpers.StartFly(fly)

		Eventually(session).Should(gexec.Exit(0))

		Expect(session).To(gbytes.Say("/hello-im-a-git-rootfs"))
		Expect(session).To(gbytes.Say("hello-im-image-provided-env"))
	})

	It("allows a version to be specified", func() {
		createFixture := func(ref string) {
			err := ioutil.WriteFile(
				filepath.Join(fixture, "task.yml"),
				[]byte(`---
platform: linux

image_resource:
  type: git
  source: {uri: "`+rootfsGitServer.URI()+`"}
  version: { ref: "`+ref+`"}

inputs:
- name: fixture

run:
  path: sh
  args:
  - -c
  - |
    touch /some-file.txt && cat /some-file.txt
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
		}

		oldRef := rootfsGitServer.RevParse("master")
		rootfsGitServer.CommitFileToBranch("hello, world", "rootfs/some-file.txt", "master")
		newRef := rootfsGitServer.RevParse("master")

		createFixture(oldRef)
		fly := exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml")
		fly.Dir = fixture

		session := helpers.StartFly(fly)

		Eventually(session).Should(gexec.Exit(0))
		Expect(session).ToNot(gbytes.Say("hello, world"))

		createFixture(newRef)
		fly = exec.Command(flyBin, "-t", targetedConcourse, "execute", "-c", "task.yml")
		fly.Dir = fixture

		session = helpers.StartFly(fly)

		Eventually(session).Should(gexec.Exit(0))
		Expect(session).To(gbytes.Say("hello, world"))
	})
})
