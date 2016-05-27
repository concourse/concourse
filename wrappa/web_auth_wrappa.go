package wrappa

import (
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

type WebAuthWrappa struct {
	Validator         auth.Validator
	UserContextReader auth.UserContextReader
}

func NewWebAuthWrappa(
	validator auth.Validator,
	userContextReader auth.UserContextReader,
) *WebAuthWrappa {
	return &WebAuthWrappa{
		Validator:         validator,
		UserContextReader: userContextReader,
	}
}

func (wrappa *WebAuthWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	for name, handler := range handlers {
		newHandler := handler

		switch name {
		case web.Index:
		case web.Pipeline:
		case web.TriggerBuild:
		case web.GetBuild:
		case web.GetBuilds:
		case web.GetJoblessBuild:
		case web.Public:
		case web.GetResource:
		case web.GetJob:
		case web.LogIn:
		case web.TeamLogIn:
		case web.GetBasicAuthLogIn:
		case web.ProcessBasicAuthLogIn:

		default:
			panic("you missed a spot")
		}

		wrapped[name] = newHandler
	}

	return wrapped
}
