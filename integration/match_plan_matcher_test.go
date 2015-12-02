package integration_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/onsi/gomega"
)

type PlanMatcher struct {
	ExpectedPlan atc.Plan
}

func MatchPlan(plan atc.Plan) *PlanMatcher {
	return &PlanMatcher{
		ExpectedPlan: plan,
	}
}

func VerifyPlan(expectedPlan atc.Plan) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var plan atc.Plan
		err := json.NewDecoder(r.Body).Decode(&plan)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		gomega.Expect(plan).To(MatchPlan(expectedPlan))
	}
}

func (matcher *PlanMatcher) Match(actual interface{}) (bool, error) {
	actualPlan, ok := actual.(atc.Plan)
	if !ok {
		return false, fmt.Errorf("expected a %T, got a %T", matcher.ExpectedPlan, actual)
	}

	return gomega.Equal(stripIDs(matcher.ExpectedPlan)).Match(stripIDs(actualPlan))
}

func (matcher *PlanMatcher) FailureMessage(actual interface{}) string {
	actualPlan := actual.(atc.Plan)
	return gomega.Equal(stripIDs(matcher.ExpectedPlan)).FailureMessage(stripIDs(actualPlan))
}

func (matcher *PlanMatcher) NegatedFailureMessage(actual interface{}) string {
	actualPlan := actual.(atc.Plan)
	return gomega.Equal(stripIDs(matcher.ExpectedPlan)).NegatedFailureMessage(stripIDs(actualPlan))
}

func stripIDs(plan atc.Plan) atc.Plan {
	plan.ID = "<stripped>"

	if plan.Aggregate != nil {
		for i, p := range *plan.Aggregate {
			(*plan.Aggregate)[i] = stripIDs(p)
		}
	}

	if plan.OnSuccess != nil {
		plan.OnSuccess.Step = stripIDs(plan.OnSuccess.Step)
		plan.OnSuccess.Next = stripIDs(plan.OnSuccess.Next)
	}

	if plan.OnFailure != nil {
		plan.OnFailure.Step = stripIDs(plan.OnFailure.Step)
		plan.OnFailure.Next = stripIDs(plan.OnFailure.Next)
	}

	if plan.Ensure != nil {
		plan.Ensure.Step = stripIDs(plan.Ensure.Step)
		plan.Ensure.Next = stripIDs(plan.Ensure.Next)
	}

	if plan.Timeout != nil {
		plan.Timeout.Step = stripIDs(plan.Timeout.Step)
	}

	if plan.Try != nil {
		plan.Try.Step = stripIDs(plan.Try.Step)
	}

	return plan
}
