package integration_test

import (
	"bytes"
	"fmt"

	"code.cloudfoundry.org/garden"
	gclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"github.com/concourse/atc"
	. "github.com/concourse/atc/cessna"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Check for new versions of resources", func() {

	var (
		checkVersions []atc.Version
		checkErr      error
	)

	Context("whose type is a base resource type", func() {
		var (
			testBaseResource Resource
			baseResourceType BaseResourceType
		)

		BeforeEach(func() {
			var (
				check string
				in    string
				out   string
			)

			check = `#!/bin/bash
			set -e
			TMPDIR=${TMPDIR:-/tmp}

			exec 3>&1 # make stdout available as fd 3 for the result
			exec 1>&2 # redirect all output to stderr for logging

			mkdir /opt/resource/logs
			logs=/opt/resource/logs/check.log
			touch $logs

			payload=$TMPDIR/echo-request
			cat > $payload <&0

			versions=$(jq -r '.source.versions // ""' < $payload)
			echo $versions >> $logs
			echo $versions >&3
			`

			c := NewResourceContainer(check, in, out)

			r, err := c.RootFSify()
			Expect(err).NotTo(HaveOccurred())

			rootFSPath, err := createBaseResourceVolume(r)
			Expect(err).ToNot(HaveOccurred())

			baseResourceType := BaseResourceType{
				RootFSPath: rootFSPath,
				Name:       "echo",
			}

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
			}.Check(logger, worker)
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

	Context("whose type is a custom resource type", func() {
		var (
			quineCheck string
			quineIn    string
			quineOut   string

			resourceCheck ResourceCheck
		)

		BeforeEach(func() {
			quineCheck = `#!/bin/bash
			set -e
			TMPDIR=${TMPDIR:-/tmp}

			exec 3>&1 # make stdout available as fd 3 for the result
			exec 1>&2 # redirect all output to stderr for logging

			payload=$TMPDIR/echo-request
			cat > $payload <&0

			versions=$(jq -r '.source.versions // ""' < $payload)
			echo $versions >&3
			`

			quineIn = `#!/bin/bash
			destination=$1

			curl ` + tarURL + ` | tar -x -C $destination

			cp -R /opt/resource $destination/opt/resource
			`

			c := NewResourceContainer(quineCheck, quineIn, quineOut)

			r, err := c.RootFSify()
			Expect(err).NotTo(HaveOccurred())

			rootFSPath, err := createBaseResourceVolume(r)
			Expect(err).ToNot(HaveOccurred())

			quineResourceType := BaseResourceType{
				RootFSPath: rootFSPath,
				Name:       "quine",
			}

			resourceCheck = ResourceCheck{
				Resource: Resource{
					ResourceType: ResourceGet{
						Resource: Resource{
							ResourceType: quineResourceType,
							Source:       atc.Source{},
						},
						Version: atc.Version{
							"beep": "boop",
						},
					},
					Source: atc.Source{
						"versions": []atc.Version{
							{"abc": "123"},
						},
					},
				},
			}
		})

		It("works", func() {
			checkResponse, checkErr := resourceCheck.Check(logger, worker)
			Expect(checkErr).NotTo(HaveOccurred())

			Expect(checkResponse).To(ContainElement(atc.Version{"abc": "123"}))
		})
	})
})
