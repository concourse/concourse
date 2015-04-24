package getbuilds_test

import (
	"time"

	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/web/getbuilds"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Present", func() {
	It("presents a list of builds", func() {
		date := time.Date(2004, 4, 3, 13, 45, 33, 0, time.UTC)

		builds := []db.Build{
			{
				ID:           1,
				JobName:      "hello",
				PipelineName: "a-pipeline",
				StartTime:    date,
				EndTime:      date.Add(1 * time.Minute),
				Status:       "pending",
				Name:         "23",
			},
			{
				ID:        2,
				JobName:   "",
				StartTime: time.Time{},
				EndTime:   date.Add(1 * time.Minute),
				Status:    "pending",
				Name:      "12",
			},
		}

		presentedBuilds := PresentBuilds(builds)

		Ω(presentedBuilds).Should(HaveLen(2))
		Ω(presentedBuilds[0]).Should(Equal(PresentedBuild{
			ID:           1,
			JobName:      "hello",
			StartTime:    "2004-04-03 13:45:33 (UTC)",
			EndTime:      "2004-04-03 13:46:33 (UTC)",
			CSSClass:     "",
			Status:       "pending",
			PipelineName: "a-pipeline",
			Path:         "/pipelines/a-pipeline/jobs/hello/builds/23",
		}))

		Ω(presentedBuilds[1]).Should(Equal(PresentedBuild{
			ID:           2,
			JobName:      "[one off]",
			StartTime:    "failed to start",
			EndTime:      "2004-04-03 13:46:33 (UTC)",
			CSSClass:     "build-one-off",
			Status:       "pending",
			PipelineName: "[one off]",
			Path:         "/builds/2",
		}))
	})
})
