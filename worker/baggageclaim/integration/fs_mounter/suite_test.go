package integration_test

import (
	"encoding/json"
	"math/rand"
	"runtime"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var fsMounterPath string

func TestIntegration(t *testing.T) {
	suiteName := "FS Mounter Suite"
	if runtime.GOOS != "linux" {
		suiteName = suiteName + " - skipping btrfs tests because non-linux"
	}

	rand.Seed(time.Now().Unix())

	RegisterFailHandler(Fail)
	RunSpecs(t, suiteName)
}

type suiteData struct {
	FSMounterPath string
}

var _ = SynchronizedBeforeSuite(func() []byte {
	fsmPath, err := gexec.Build("github.com/concourse/concourse/worker/baggageclaim/cmd/fs_mounter", "-buildvcs=false")
	Expect(err).NotTo(HaveOccurred())

	data, err := json.Marshal(suiteData{
		FSMounterPath: fsmPath,
	})
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	var suiteData suiteData
	err := json.Unmarshal(data, &suiteData)
	Expect(err).NotTo(HaveOccurred())

	fsMounterPath = suiteData.FSMounterPath
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})
