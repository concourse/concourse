package wrappa

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/concourse/concourse/atc"
)

type ConcurrentRequestLimitFlag struct {
	Action string
	Limit  int
}

func (crl *ConcurrentRequestLimitFlag) UnmarshalFlag(value string) error {
	variable, expression, err := parseAssignment(value)
	if err != nil {
		return err
	}
	limit, err := strconv.Atoi(expression)
	if err != nil || limit < 0 {
		return formatError(value, "limit must be a non-negative integer")
	}
	if !isValidAction(variable) {
		return formatError(
			value,
			fmt.Sprintf("'%s' is not a valid action", variable),
		)
	}
	crl.Action = variable
	crl.Limit = limit
	return nil
}

func parseAssignment(value string) (string, string, error) {
	assignment := strings.Split(value, "=")
	if len(assignment) != 2 {
		return "", "", formatError(value, "value must be an assignment")
	}
	return assignment[0], assignment[1], nil
}

func formatError(value string, reason string) error {
	return fmt.Errorf("invalid concurrent request limit '%s': %s", value, reason)
}

func isValidAction(action string) bool {
	for _, route := range atc.Routes {
		if route.Name == action {
			return true
		}
	}
	return false
}

type ConcurrentRequestPolicy interface {
	MaxConcurrentRequests(action string) int
	IsLimited(action string) bool
	Validate() error
}

type concurrentRequestPolicy struct {
	limits           []ConcurrentRequestLimitFlag
	supportedActions []string
}

func NewConcurrentRequestPolicy(
	limits []ConcurrentRequestLimitFlag,
	supportedActions []string,
) ConcurrentRequestPolicy {
	return &concurrentRequestPolicy{
		limits:           limits,
		supportedActions: supportedActions,
	}
}

func (crp *concurrentRequestPolicy) MaxConcurrentRequests(action string) int {
	return 0
}

func (crp *concurrentRequestPolicy) IsLimited(action string) bool {
	return false
}

func (crp *concurrentRequestPolicy) Validate() error {
	err := crp.checkSupportedActions()
	if err != nil {
		return err
	}
	err = crp.checkDuplicateActions()
	if err != nil {
		return err
	}
	return nil
}

func (crp *concurrentRequestPolicy) checkSupportedActions() error {
	for _, limit := range crp.limits {
		if !crp.supports(limit.Action) {
			return formatError(
				fmt.Sprintf("%s=%d", limit.Action, limit.Limit),
				fmt.Sprintf("'%s' is not supported", limit.Action),
			)
		}
	}
	return nil
}

func (crp *concurrentRequestPolicy) checkDuplicateActions() error {
	counter := map[string]bool{}
	for _, limit := range crp.limits {
		if counter[limit.Action] {
			return fmt.Errorf(
				"invalid concurrent request limits: multiple limits on '%s'",
				limit.Action,
			)
		}
		counter[limit.Action] = true
	}
	return nil
}

func (crp *concurrentRequestPolicy) supports(action string) bool {
	for _, supportedAction := range crp.supportedActions {
		if action == supportedAction {
			return true
		}
	}
	return false
}

