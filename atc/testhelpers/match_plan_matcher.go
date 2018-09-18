package testhelpers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

type PlanMatcher struct {
	ExpectedPlan atc.Plan

	failedMatcher types.GomegaMatcher
	failedValue   interface{}
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

	expectedStripped, expectedIDs := stripIDs(matcher.ExpectedPlan)
	actualStripped, actualIDs := stripIDs(actualPlan)

	planMatcher := gomega.Equal(expectedStripped)
	idsMatcher := gomega.ConsistOf(expectedIDs)

	matched, err := planMatcher.Match(actualStripped)
	if err != nil {
		return false, err
	}

	if !matched {
		matcher.failedMatcher = planMatcher
		matcher.failedValue = actualStripped
		return false, nil
	}

	matched, err = idsMatcher.Match(actualIDs)
	if err != nil {
		return false, err
	}

	if !matched {
		matcher.failedMatcher = idsMatcher
		matcher.failedValue = actualIDs
		return false, nil
	}

	return true, nil
}

func (matcher *PlanMatcher) FailureMessage(actual interface{}) string {
	return matcher.failedMatcher.FailureMessage(matcher.failedValue)
}

func (matcher *PlanMatcher) NegatedFailureMessage(actual interface{}) string {
	return matcher.failedMatcher.NegatedFailureMessage(matcher.failedValue)
}

func stripIDs(plan atc.Plan) (atc.Plan, []string) {
	var ids []string

	var subIDs []string

	plan.ID = "<stripped>"

	if plan.Aggregate != nil {
		for i, p := range *plan.Aggregate {
			(*plan.Aggregate)[i], subIDs = stripIDs(p)
			ids = append(ids, subIDs...)
		}
	}

	if plan.Do != nil {
		for i, p := range *plan.Do {
			(*plan.Do)[i], subIDs = stripIDs(p)
			ids = append(ids, subIDs...)
		}
	}

	if plan.OnSuccess != nil {
		plan.OnSuccess.Step, subIDs = stripIDs(plan.OnSuccess.Step)
		ids = append(ids, subIDs...)

		plan.OnSuccess.Next, subIDs = stripIDs(plan.OnSuccess.Next)
		ids = append(ids, subIDs...)
	}

	if plan.OnFailure != nil {
		plan.OnFailure.Step, subIDs = stripIDs(plan.OnFailure.Step)
		ids = append(ids, subIDs...)

		plan.OnFailure.Next, subIDs = stripIDs(plan.OnFailure.Next)
		ids = append(ids, subIDs...)
	}

	if plan.Ensure != nil {
		plan.Ensure.Step, subIDs = stripIDs(plan.Ensure.Step)
		ids = append(ids, subIDs...)

		plan.Ensure.Next, subIDs = stripIDs(plan.Ensure.Next)
		ids = append(ids, subIDs...)
	}

	if plan.Timeout != nil {
		plan.Timeout.Step, subIDs = stripIDs(plan.Timeout.Step)
		ids = append(ids, subIDs...)
	}

	if plan.Try != nil {
		plan.Try.Step, subIDs = stripIDs(plan.Try.Step)
		ids = append(ids, subIDs...)
	}

	if plan.Get != nil {
		if plan.Get.VersionFrom != nil {
			planID := atc.PlanID("<stripped>")
			plan.Get.VersionFrom = &planID
		}
	}

	return plan, ids
}
