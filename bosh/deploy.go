package bosh

import (
	"io/ioutil"
	"os/exec"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Deploy(deploymentName string, templateData ...interface{}) {
	var deploymentPath string

	if len(templateData) > 0 {
		deploymentTemplate, err := template.ParseFiles(deploymentName)
		Ω(err).ShouldNot(HaveOccurred())

		deploymentFile, err := ioutil.TempFile("", "deployment")
		Ω(err).ShouldNot(HaveOccurred())

		err = deploymentTemplate.Execute(deploymentFile, templateData[0])
		Ω(err).ShouldNot(HaveOccurred())

		err = deploymentFile.Close()
		Ω(err).ShouldNot(HaveOccurred())

		deploymentPath = deploymentFile.Name()
	} else {
		deploymentPath = deploymentName
	}

	run(exec.Command("bosh", "deployment", deploymentPath))
	run(exec.Command("bosh", "-n", "deploy"))
}

func run(cmd *exec.Cmd) {
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	err := cmd.Run()
	Ω(err).ShouldNot(HaveOccurred())
}
