package integration_test

import (
	"archive/tar"
	"io/ioutil"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/cessna/resource"
	"github.com/concourse/baggageclaim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Get version of a resource", func() {

	var getVolume baggageclaim.Volume
	var getErr error

	var (
		check string
		in    string
		out   string
	)

	Context("whose type is a base resource type", func() {

		BeforeEach(func() {
			in = `#!/bin/bash
			set -e
			TMPDIR=${TMPDIR:-/tmp}

			exec 3>&1 # make stdout available as fd 3 for the result
			exec 1>&2 # redirect all output to stderr for logging

			destination=$1

			mkdir -p $destination

			payload=$TMPDIR/echo-request
			cat > $payload <&0

			version=$(jq -r '.version // ""' < $payload)

			echo $version > $destination/version

			echo '{ "version" : {}, "metadata": []  }' >&3
			`

			c := NewResourceContainer(check, in, out)

			r, err := c.RootFSify()
			Expect(err).NotTo(HaveOccurred())

			rootFSPath, err := createBaseResourceVolume(r)
			Expect(err).ToNot(HaveOccurred())

			baseResourceType = BaseResourceType{
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
			getVolume, getErr = ResourceGet{
				Resource: testBaseResource,
				Version:  atc.Version{"beep": "boop"},
				Params:   nil,
			}.Get(logger, testWorker)
		})

		It("runs the get script", func() {
			Expect(getErr).ShouldNot(HaveOccurred())
		})

		It("returns a volume with the result of running the get script", func() {
			file, err := getVolume.StreamOut("/version")
			Expect(err).ToNot(HaveOccurred())
			defer file.Close()

			tarReader := tar.NewReader(file)
			tarReader.Next()

			bytes, err := ioutil.ReadAll(tarReader)
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes).To(MatchJSON(`{"beep": "boop"}`))
		})

	})

	Context("whose type is a custom resource type", func() {
		var (
			quineCheck string
			quineIn    string
			quineOut   string

			resourceGet ResourceGet
		)

		BeforeEach(func() {
			quineIn = `#!/bin/bash

			TMPDIR=${TMPDIR:-/tmp}

			exec 3>&1 # make stdout available as fd 3 for the result
			exec 1>&2 # redirect all output to stderr for logging

			payload=$TMPDIR/request
			cat > $payload <&0

			destination=$1

			cp -a / $destination/ || true

			version=$(jq -r '.version // ""' < $payload)

			echo $version > $destination/version
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

			resourceGet = ResourceGet{
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
				Version: atc.Version{
					"yellow": "blue",
				},
			}
		})

		It("works", func() {
			getVolume, getErr := resourceGet.Get(logger, testWorker)
			Expect(getErr).NotTo(HaveOccurred())

			file, err := getVolume.StreamOut("/version")
			Expect(err).ToNot(HaveOccurred())
			defer file.Close()

			tarReader := tar.NewReader(file)
			tarReader.Next()

			bytes, err := ioutil.ReadAll(tarReader)
			Expect(err).NotTo(HaveOccurred())
			Expect(bytes).To(MatchJSON(`{"yellow": "blue"}`))
		})
	})
})
