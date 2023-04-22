package present_test

import (
	"fmt"

	"github.com/concourse/concourse/atc/api/accessor"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/concourse/atc/api/present"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Build", func() {
	var dbBuild dbfakes.FakeBuild

	Describe("Comments", func() {
		var comment string = "🎉 Comments Work! 🥳"

		checkComment := func(expect bool, job db.Job, access accessor.Access) {
			build := present.Build(&dbBuild, job, access)

			if expect {
				Expect(build.Comment).To(Equal(comment))
			} else {
				Expect(build.Comment).To(BeEmpty())
			}
		}

		BeforeEach(func() {
			dbBuild.CommentReturns(comment)
		})

		It("should not be set if neither job nor accessor is passed in", func() {
			checkComment(false, nil, nil)
		})

		for _, v := range []bool{false, true} {
			It(fmt.Sprintf("should be set if job is public (%v)", v), func() {
				var dbJob dbfakes.FakeJob
				dbJob.PublicReturns(v)

				checkComment(v, &dbJob, nil)
			})

			It(fmt.Sprintf("should be set if accessor allows it (%v)", v), func() {
				var accessor accessorfakes.FakeAccess
				accessor.IsAuthorizedReturns(v)

				checkComment(v, nil, &accessor)
			})
		}
	})
})
