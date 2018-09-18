package engine_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/concourse/atc/engine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildDelegate", func() {
	var (
		factory BuildDelegateFactory

		fakeBuild *dbfakes.FakeBuild

		delegate BuildDelegate

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		factory = NewBuildDelegateFactory()

		fakeBuild = new(dbfakes.FakeBuild)
		delegate = factory.Delegate(fakeBuild)

		logger = lagertest.NewTestLogger("test")
	})

	Describe("Finish", func() {
		Context("when build was aborted", func() {
			BeforeEach(func() {
				delegate.Finish(logger, context.Canceled, false)
			})

			It("updates build status to aborted", func() {
				finishedStatus := fakeBuild.FinishArgsForCall(0)
				Expect(finishedStatus).To(Equal(db.BuildStatusAborted))
			})
		})

		Context("when build had error", func() {
			BeforeEach(func() {
				delegate.Finish(logger, errors.New("disaster"), false)
			})

			It("updates build status to errorred", func() {
				finishedStatus := fakeBuild.FinishArgsForCall(0)
				Expect(finishedStatus).To(Equal(db.BuildStatusErrored))
			})
		})
	})
})
