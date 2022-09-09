package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Fly CLI", func() {
	Describe("format-pipeline", func() {
		var (
			configFile   *os.File
			inputYaml    []byte
			expectedYaml []byte
		)

		BeforeEach(func() {
			var err error
			configFile, err = ioutil.TempFile("", "format-pipeline-test-*.yml")
			Expect(err).NotTo(HaveOccurred())

			inputYaml, err = ioutil.ReadFile("fixtures/format-input.yml")
			Expect(err).NotTo(HaveOccurred())

			expectedYaml, err = ioutil.ReadFile("fixtures/format-expected.yml")
			Expect(err).NotTo(HaveOccurred())

			_, err = configFile.Write(inputYaml)
			Expect(err).NotTo(HaveOccurred())

			err = configFile.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := os.RemoveAll(configFile.Name())
			Expect(err).NotTo(HaveOccurred())
		})

		It("prints the formatted pipeline YAML to stdout", func() {
			flyCmd := exec.Command(
				flyPath,
				"format-pipeline",
				"-c", configFile.Name(),
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			Expect(sess.Out.Contents()).To(MatchYAML(expectedYaml))
		})

		It("preserves the original pipeline file", func() {
			oldFileInfo, err := os.Stat(configFile.Name())
			Expect(err).NotTo(HaveOccurred())

			flyCmd := exec.Command(
				flyPath,
				"format-pipeline",
				"-c", configFile.Name(),
			)

			sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			<-sess.Exited
			Expect(sess.ExitCode()).To(Equal(0))

			newFileInfo, err := os.Stat(configFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(newFileInfo.ModTime()).To(Equal(oldFileInfo.ModTime()))

			newYaml, err := ioutil.ReadFile(configFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(newYaml).To(Equal(inputYaml))
		})

		Context("when given the -w option", func() {
			It("overwrites the file in-place", func() {
				flyCmd := exec.Command(
					flyPath,
					"format-pipeline",
					"-c", configFile.Name(),
					"-w",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				newYaml, err := ioutil.ReadFile(configFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(newYaml).To(MatchYAML(expectedYaml))
			})

			It("is idempotent", func() {
				flyCmd := exec.Command(
					flyPath,
					"format-pipeline",
					"-c", configFile.Name(),
					"-w",
				)

				sess, err := gexec.Start(flyCmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess.Exited
				Expect(sess.ExitCode()).To(Equal(0))

				firstPassYaml, err := ioutil.ReadFile(configFile.Name())
				Expect(err).NotTo(HaveOccurred())

				flyCmd2 := exec.Command(
					flyPath,
					"format-pipeline",
					"-c", configFile.Name(),
					"-w",
				)

				sess2, err := gexec.Start(flyCmd2, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())

				<-sess2.Exited
				Expect(sess2.ExitCode()).To(Equal(0))

				secondPassYaml, err := ioutil.ReadFile(configFile.Name())
				Expect(err).NotTo(HaveOccurred())

				Expect(firstPassYaml).To(MatchYAML(secondPassYaml))
			})
		})
	})
})
