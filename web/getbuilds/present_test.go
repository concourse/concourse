package getbuilds_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web/getbuilds"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Present", func() {
	It("presents a list of builds", func() {
		date := time.Date(2004, 4, 3, 13, 45, 33, 0, time.UTC)

		builds := []atc.Build{
			{
				ID:           1,
				JobName:      "hello",
				PipelineName: "a-pipeline",
				TeamName:     "a-team",
				StartTime:    date.Unix(),
				EndTime:      date.Add(1 * time.Minute).Unix(),
				Status:       "pending",
				Name:         "23",
			},
			{
				ID:        2,
				JobName:   "",
				StartTime: 0,
				EndTime:   0,
				Status:    "pending",
				Name:      "12",
				TeamName:  "a-team",
			},
		}

		presentedBuilds := getbuilds.PresentBuilds(builds)

		Expect(presentedBuilds).To(HaveLen(2))
		Expect(presentedBuilds[0]).To(Equal(getbuilds.PresentedBuild{
			ID:           1,
			JobName:      "hello",
			StartTime:    "2004-04-03 13:45:33 (UTC)",
			EndTime:      "2004-04-03 13:46:33 (UTC)",
			CSSClass:     "",
			Status:       "pending",
			PipelineName: "a-pipeline",
			TeamName:     "a-team",
			Path:         "/teams/a-team/pipelines/a-pipeline/jobs/hello/builds/23",
		}))

		Expect(presentedBuilds[1]).To(Equal(getbuilds.PresentedBuild{
			ID:           2,
			JobName:      "[one off]",
			StartTime:    "n/a",
			EndTime:      "n/a",
			CSSClass:     "build-one-off",
			Status:       "pending",
			PipelineName: "[one off]",
			TeamName:     "a-team",
			Path:         "/builds/2",
		}))
	})
})
