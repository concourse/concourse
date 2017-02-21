package integration_test

import (
	"archive/tar"
	"io/ioutil"

	"github.com/concourse/atc"
	. "github.com/concourse/atc/cessna"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Get version of a resource", func() {
	Context("whose type is a base resource type", func() {
		var testBaseResource Resource

		BeforeEach(func() {
			in := `#!/bin/bash
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
			check := ""
			out := ""

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

		It("returns a volume with the result of running the get script", func() {
			getVolume, getErr := ResourceGet{
				Resource: testBaseResource,
				Version:  atc.Version{"beep": "boop"},
				Params:   nil,
			}.Get(logger, worker)
			Expect(getErr).ShouldNot(HaveOccurred())

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
		var resourceGet ResourceGet

		BeforeEach(func() {
			quineIn := `#!/bin/bash
			set -eux

			TMPDIR=${TMPDIR:-/tmp}

			exec 3>&1 # make stdout available as fd 3 for the result
			exec 1>&2 # redirect all output to stderr for logging

			payload=$TMPDIR/request
			cat > $payload <&0

			destination=$1

			curl ` + tarURL + ` | tar -x -C $destination

			version=$(jq -r '.version // ""' < $payload)

			cp -R /opt/resource $destination/opt/resource
			echo $version > $destination/version
			`
			quineCheck := ""
			quineOut := ""
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
			getVolume, getErr := resourceGet.Get(logger, worker)
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
