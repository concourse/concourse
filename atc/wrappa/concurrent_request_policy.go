package wrappa

import (
	"fmt"

	"github.com/concourse/concourse/atc"
)

type LimitedRoute string

var supportedActions = []LimitedRoute{LimitedRoute(atc.ListAllJobs)}

func (lr *LimitedRoute) UnmarshalFlag(value string) error {
	if !isValidAction(value) {
		return fmt.Errorf("'%s' is not a valid action", value)
	}
	for _, supportedAction := range supportedActions {
		if value == string(supportedAction) {
			*lr = supportedAction
			return nil
		}
	}
	return fmt.Errorf(
		"action '%s' is not supported. Supported actions are: %v",
		value,
		supportedActions,
	)
}

func isValidAction(action string) bool {
	for _, route := range atc.Routes {
		if route.Name == action {
			return true
		}
	}
	return false
}

//go:generate counterfeiter . ConcurrentRequestPolicy

type ConcurrentRequestPolicy interface {
	HandlerPool(action string) (Pool, bool)
}

type concurrentRequestPolicy struct {
	handlerPools map[LimitedRoute]Pool
}

func NewConcurrentRequestPolicy(
	limits map[LimitedRoute]int,
) ConcurrentRequestPolicy {
	pools := map[LimitedRoute]Pool{}
	for action, limit := range limits {
		pools[action] = NewPool(limit)
	}
	return &concurrentRequestPolicy{
		handlerPools: pools,
	}
}

func (crp *concurrentRequestPolicy) HandlerPool(action string) (Pool, bool) {
	pool, found := crp.handlerPools[LimitedRoute(action)]
	return pool, found
}
