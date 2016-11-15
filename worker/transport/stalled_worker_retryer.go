package transport

import "github.com/concourse/retryhttp"

type StalledWorkerRetryer struct {
	DelegateRetryer retryhttp.Retryer
}

func (r *StalledWorkerRetryer) IsRetryable(err error) bool {
	if _, ok := err.(ErrWorkerStalled); ok {
		return true
	}

	return r.DelegateRetryer.IsRetryable(err)
}
