package bosh

import (
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func DeployConcourse(deploymentName string, templateData ...interface{}) Deployment {
	var deploymentPath string

	if len(templateData) > 0 {
		deploymentTemplate, err := template.ParseFiles(deploymentName)
		Ω(err).ShouldNot(HaveOccurred())

		deploymentFile, err := ioutil.TempFile("", "deployment")
		Ω(err).ShouldNot(HaveOccurred())

		err = deploymentTemplate.Execute(deploymentFile, templateData)
		Ω(err).ShouldNot(HaveOccurred())

		err = deploymentFile.Close()
		Ω(err).ShouldNot(HaveOccurred())

		deploymentPath = deploymentFile.Name()
	} else {
		deploymentPath = deploymentName
	}

	run(exec.Command("bosh", "deployment", deploymentPath))
	run(exec.Command("bosh", "-n", "deploy"))

	return Deployment{
		ATCUrl: "http://" + os.Getenv("BOSH_LITE_IP") + ":8080",
	}
}

func run(cmd *exec.Cmd) {
	cmd.Stdout = GinkgoWriter
	cmd.Stderr = GinkgoWriter

	err := cmd.Run()
	Ω(err).ShouldNot(HaveOccurred())
}
