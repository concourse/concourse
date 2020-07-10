package testhelpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

var (
	ErrPlanWithoutMatchingResourceTypes = errors.New("Found a plan without matching ResourceTypes")
)

type HasResourceTypesMatcher struct {
	matcher     types.GomegaMatcher
	failedValue interface{}
}

func MatchHasResourceTypes(resourceTypes atc.VersionedResourceTypes) *HasResourceTypesMatcher {
	return &HasResourceTypesMatcher{
		matcher: gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"VersionedResourceTypes": gomega.ConsistOf(resourceTypes),
		}),
	}
}

func VerifyResourceTypes(resourceTypes ...atc.VersionedResourceType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var plan atc.Plan
		err := json.NewDecoder(r.Body).Decode(&plan)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		gomega.Expect(plan).To(MatchHasResourceTypes(resourceTypes))
	}
}

func (m *HasResourceTypesMatcher) hasResourceTypes(p *atc.Plan) (bool, error) {
	var matched bool
	var err error

	switch {
	case p.Get != nil:
		matched, err = m.matcher.Match(*p.Get)
	case p.Put != nil:
		matched, err = m.matcher.Match(*p.Put)
	case p.Task != nil:
		matched, err = m.matcher.Match(*p.Task)
	case p.Check != nil:
		matched, err = m.matcher.Match(*p.Check)
	default:
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if !matched {
		m.failedValue = p
	}

	return true, nil
}

func (matcher *HasResourceTypesMatcher) Match(actual interface{}) (bool, error) {
	actualPlan, ok := actual.(atc.Plan)
	if !ok {
		return false, fmt.Errorf("expected a atc.Plan, got a %T", actual)
	}

	matched := true
	var iterErr error

	actualPlan.Each(func(plan *atc.Plan) {
		m, err := matcher.hasResourceTypes(plan)
		if err != nil {
			iterErr = err
			return
		}

		if !m {
			// exit early
			matched = false
		}
	})

	return matched, iterErr
}

func (matcher *HasResourceTypesMatcher) FailureMessage(actual interface{}) string {
	return matcher.matcher.FailureMessage(matcher.failedValue)
}

func (matcher *HasResourceTypesMatcher) NegatedFailureMessage(actual interface{}) string {
	return matcher.matcher.NegatedFailureMessage(matcher.failedValue)
}
