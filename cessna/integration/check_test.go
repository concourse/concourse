package integration_test

import (
	"bytes"
	"fmt"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"github.com/concourse/atc"
	. "github.com/concourse/atc/cessna/resource"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Check for new versions of resources", func() {

	var checkVersions []atc.Version
	var checkErr error

	Context("whose type is a base resource type", func() {

		BeforeEach(func() {
			source := atc.Source{
				"versions": []map[string]string{
					{"ref": "123"},
					{"beep": "boop"},
				},
			}

			testBaseResource = NewBaseResource(baseResourceType, source)
		})

		JustBeforeEach(func() {
			checkVersions, checkErr = ResourceCheck{
				Resource: testBaseResource,
				Version:  nil,
			}.Check(logger, testWorker)
		})

		It("runs the check script", func() {
			Expect(checkErr).ShouldNot(HaveOccurred())
		})

		It("returns the proper versions", func() {
			Expect(checkVersions).To(ConsistOf(atc.Version{"ref": "123"}, atc.Version{"beep": "boop"}))
		})

		It("runs everything using a COW volume", func() {
			gardenClient := gclient.New(connection.New("tcp", fmt.Sprintf("%s:7777", workerIp)))

			container, err := gardenClient.Create(garden.ContainerSpec{
				RootFSPath: baseResourceType.RootFSPath,
			})

			stdout := gbytes.NewBuffer()
			stderr := new(bytes.Buffer)

			Expect(err).ToNot(HaveOccurred())
			proc, err := container.Run(
				garden.ProcessSpec{
					Path: "ls",
					Dir:  "/opt/resource/logs/",
				},
				garden.ProcessIO{
					Stderr: stderr,
					Stdout: stdout,
				})
			Expect(err).ToNot(HaveOccurred())
			proc.Wait()
			Expect(string(stdout.Contents())).ToNot(ContainSubstring("check.log"))
		})

	})

})
