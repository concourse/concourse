package policy_test

import (
	"context"
	"github.com/concourse/concourse/atc/policy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Context("PolicyContext", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("should set and get team and pipeline", func() {
		newCtx := policy.RecordTeamAndPipeline(ctx, "some-team", "some-pipeline")
		Expect(newCtx).ToNot(BeNil())

		team, pipeline := policy.TeamAndPipelineFromContext(newCtx)
		Expect(team).To(Equal("some-team"))
		Expect(pipeline).To(Equal("some-pipeline"))
	})
})
