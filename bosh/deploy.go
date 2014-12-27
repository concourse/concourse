package bosh

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"text/template"

	"github.com/mgutz/ansi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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
	cmd.Stdout = gexec.NewPrefixedWriter(
		fmt.Sprintf("%s%s ", ansi.Color("[o]", "green"), ansi.Color("[bosh]", "black+bright")),
		GinkgoWriter,
	)

	cmd.Stderr = gexec.NewPrefixedWriter(
		fmt.Sprintf("%s%s ", ansi.Color("[e]", "red+bright"), ansi.Color("[bosh]", "black+bright")),
		GinkgoWriter,
	)

	err := cmd.Run()
	Ω(err).ShouldNot(HaveOccurred())
}
