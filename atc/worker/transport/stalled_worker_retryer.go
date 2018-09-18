package transport

import "github.com/concourse/retryhttp"

type UnreachableWorkerRetryer struct {
	DelegateRetryer retryhttp.Retryer
}

func (r *UnreachableWorkerRetryer) IsRetryable(err error) bool {
	if _, ok := err.(WorkerUnreachableError); ok {
		return true
	}

	return r.DelegateRetryer.IsRetryable(err)
}
