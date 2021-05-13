package wrappa

import (
	"github.com/concourse/concourse/atc"
)

var SupportedActions = []string{atc.ListAllJobs}

//counterfeiter:generate . ConcurrentRequestPolicy
type ConcurrentRequestPolicy interface {
	HandlerPool(action string) (Pool, bool)
}

type concurrentRequestPolicy struct {
	handlerPools map[string]Pool
}

func NewConcurrentRequestPolicy(
	limits map[string]int,
) ConcurrentRequestPolicy {
	pools := map[string]Pool{}
	for action, limit := range limits {
		pools[action] = NewPool(limit)
	}
	return &concurrentRequestPolicy{
		handlerPools: pools,
	}
}

func (crp *concurrentRequestPolicy) HandlerPool(action string) (Pool, bool) {
	pool, found := crp.handlerPools[action]
	return pool, found
}
