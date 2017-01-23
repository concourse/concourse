package integration_test

import (
	"archive/tar"
	"io/ioutil"

	. "github.com/concourse/atc/cessna"
	"github.com/concourse/atc/cessna/cessnafakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/go-archive/archivetest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Running a task", func() {
	It("it mounts the inputs as COWs and the outputs directly", func() {
		rootFSgenerator := new(cessnafakes.FakeRootFSable)

		rootFSgenerator.RootFSPathForReturns("docker:///alpine", nil)

		inputVolume, err := worker.BaggageClaimClient().CreateVolume(
			logger,
			"some-input-volume-handle",
			baggageclaim.VolumeSpec{
				Strategy: baggageclaim.EmptyStrategy{},
			},
		)
		Expect(err).NotTo(HaveOccurred())

		outputVolume, err := worker.BaggageClaimClient().CreateVolume(
			logger,
			"some-output-volume-handle",
			baggageclaim.VolumeSpec{
				Strategy: baggageclaim.EmptyStrategy{},
			},
		)
		Expect(err).NotTo(HaveOccurred())

		a := archivetest.Archive{
			{
				Name: "file",
				Body: "hello",
			},
		}

		s, err := a.TarStream()
		Expect(err).NotTo(HaveOccurred())

		inputVolume.StreamIn(".", s)

		inputArtifacts := make(NamedArtifacts)
		outputArtifacts := make(NamedArtifacts)

		inputArtifacts["./input1"] = inputVolume

		outputArtifacts["./output1"] = outputVolume

		task := Task{
			RootFSGenerator: rootFSgenerator,
			Path:            "/bin/sh",
			Args:            []string{"-c", "cp $INPUT_FILE $OUTPUT_FILE"},
			Env: []string{
				"INPUT_FILE=input1/file",
				"OUTPUT_FILE=output1/otherfile",
			},
		}

		err = task.Run(logger, worker, inputArtifacts, outputArtifacts)
		Expect(err).NotTo(HaveOccurred())

		file, err := outputVolume.StreamOut("otherfile")
		Expect(err).NotTo(HaveOccurred())

		defer file.Close()

		tarReader := tar.NewReader(file)
		tarReader.Next()

		bytes, err := ioutil.ReadAll(tarReader)
		Expect(err).NotTo(HaveOccurred())
		Expect(bytes).To(Equal([]byte(`hello`)))
	})
})
