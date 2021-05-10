package wrappa

import "github.com/tedsuo/rata"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate net/http.Handler

type Wrappa interface {
	Wrap(rata.Handlers) rata.Handlers
}
