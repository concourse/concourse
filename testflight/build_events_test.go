package testflight_test

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Build Events", func() {
	It("across steps log errors from sub-steps", func() {
		setAndUnpausePipeline("fixtures/erroring_pipeline.yml")
		sess := spawnFly("trigger-job", "-j", inPipeline("across-step"), "-w")
		wait(sess, true)

		sess = spawnFly("builds", "-j", inPipeline("across-step"), "--json")
		wait(sess, false)

		buildsRaw := sess.Out.Contents()
		builds := []struct {
			ApiUrl string `json:"api_url"`
		}{}
		json.Unmarshal(buildsRaw, &builds)

		sess = spawnFly("curl", builds[0].ApiUrl+"/events", "--", "--max-time", "5")
		wait(sess, true)

		buildEvents := sess.Out.Contents()
		errEvents := strings.Count(string(buildEvents), `"event":"error"`)
		Expect(errEvents).To(Equal(2))
	})
})
