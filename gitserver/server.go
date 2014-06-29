package gitserver

import (
	"fmt"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/nu7hatch/gouuid"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var container warden.Container

var ipAddress string

var committedGuids []string

func Start(wardenClient warden.Client) {
	var err error

	container, err = wardenClient.Create(warden.ContainerSpec{
		RootFSPath: "docker:///concourse/testflight-helper",
	})
	Ω(err).ShouldNot(HaveOccurred())

	info, err := container.Info()
	Ω(err).ShouldNot(HaveOccurred())

	ipAddress = info.ContainerIP

	_, stream, err := container.Run(warden.ProcessSpec{
		Script: `
git config --global user.email dummy@example.com
git config --global user.name "Dummy User"

mkdir some-repo
cd some-repo
git init
touch .git/git-daemon-export-ok
`,
	})

	for chunk := range stream {
		ginkgo.GinkgoWriter.Write(chunk.Data)

		if chunk.ExitStatus != nil {
			Ω(*chunk.ExitStatus).Should(BeZero())
		}
	}

	_, stream, err = container.Run(warden.ProcessSpec{
		Script: "git daemon --reuseaddr --base-path=$HOME --detach $HOME",
	})

	for chunk := range stream {
		ginkgo.GinkgoWriter.Write(chunk.Data)

		if chunk.ExitStatus != nil {
			Ω(*chunk.ExitStatus).Should(BeZero())
		}
	}
}

func Stop(wardenClient warden.Client) {
	wardenClient.Destroy(container.Handle())

	container = nil
	ipAddress = ""
}

func URI() string {
	return fmt.Sprintf("git://%s/some-repo", ipAddress)
}

func Commit() {
	guid, err := uuid.NewV4()
	Ω(err).ShouldNot(HaveOccurred())

	_, stream, err := container.Run(warden.ProcessSpec{
		Script: `
cd some-repo
echo '%s' >> guids
git add guids
git commit -m "$(date)"
`,
	})

	for chunk := range stream {
		ginkgo.GinkgoWriter.Write(chunk.Data)

		if chunk.ExitStatus != nil {
			Ω(*chunk.ExitStatus).Should(BeZero())
		}
	}

	committedGuids = append(committedGuids, guid.String())
}

func CommittedGuids() []string {
	return committedGuids
}
