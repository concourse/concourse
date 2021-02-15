package testhelpers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"
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

	expectedStripped, _ := stripIDs(matcher.ExpectedPlan)
	actualStripped, actualIDs := stripIDs(actualPlan)

	planMatcher := gomega.Equal(expectedStripped)

	if !idsAreUnique(actualIDs) {
		return false, fmt.Errorf("expected %#v to contain unique elements", actualIDs)
	}

	matched, err := planMatcher.Match(actualStripped)
	if err != nil {
		return false, err
	}

	if !matched {
		matcher.failedMatcher = planMatcher
		matcher.failedValue = actualStripped
		return false, nil
	}

	return true, nil
}

func idsAreUnique(ids []string) bool {
	seenIds := make(map[string]bool)

	for _, id := range ids {
		if seenIds[id] {
			return false
		}

		seenIds[id] = true
	}

	return true
}

func (matcher *PlanMatcher) FailureMessage(actual interface{}) string {
	return matcher.failedMatcher.FailureMessage(matcher.failedValue)
}

func (matcher *PlanMatcher) NegatedFailureMessage(actual interface{}) string {
	return matcher.failedMatcher.NegatedFailureMessage(matcher.failedValue)
}

func stripIDs(plan atc.Plan) (atc.Plan, []string) {
	ids := []string{}

	// Ignore errors, since our walker doesn't return an error.
	plan.Each(func(plan *atc.Plan) {
		ids = append(ids, string(plan.ID))

		plan.ID = "<stripped>"

		if plan.Get != nil {
			if plan.Get.VersionFrom != nil {
				planID := atc.PlanID("<stripped>")
				plan.Get.VersionFrom = &planID
			}
		}
	})

	return plan, ids
}
