package wrappa

import "github.com/tedsuo/rata"

type Wrappa interface {
	Wrap(rata.Handlers) rata.Handlers
}

//go:generate counterfeiter net/http.Handler
