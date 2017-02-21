package integration_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/atc/cessna"
	"github.com/concourse/baggageclaim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Put a resource", func() {

	var (
		getBaseResource Resource
		getVolume       baggageclaim.Volume
		getErr          error

		putBaseResource Resource
		putResponse     OutResponse
		putErr          error

		check string
		in    string
		out   string

		baseResourceType BaseResourceType
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

			out = `#!/bin/bash
			set -e
			TMPDIR=${TMPDIR:-/tmp}

			exec 3>&1 # make stdout available as fd 3 for the result
			exec 1>&2 # redirect all output to stderr for logging

			inputs=$1

			payload=$TMPDIR/echo-request
			cat > $payload <&0

			versionPath=$(jq -r '.params.path' < $payload)

			cd "${inputs}"
			version=$(cat $versionPath)

			echo "{ \"version\" : ${version}, \"metadata\": []}" >&3
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

			getBaseResource = NewBaseResource(baseResourceType, source)

			getVolume, getErr = ResourceGet{Resource: getBaseResource, Version: atc.Version{"beep": "boop"}}.Get(logger, worker)

			putBaseResource = NewBaseResource(baseResourceType, source)
		})

		JustBeforeEach(func() {
			putResponse, putErr = ResourcePut{
				Resource: putBaseResource,
				Params: atc.Params{
					"path": "inputresource/version",
				},
			}.Put(logger, worker, NamedArtifacts{
				"inputresource": getVolume,
			})
		})

		It("runs the out script", func() {
			Expect(putErr).ShouldNot(HaveOccurred())
			Expect(putResponse.Version).To(Equal(atc.Version{"beep": "boop"}))
		})
	})

})
