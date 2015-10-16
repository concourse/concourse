package wrappa

import (
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web"
	"github.com/tedsuo/rata"
)

type WebAuthWrappa struct {
	PubliclyViewable bool
	Validator        auth.Validator
}

func NewWebAuthWrappa(
	publiclyViewable bool,
	validator auth.Validator,
) *WebAuthWrappa {
	return &WebAuthWrappa{
		PubliclyViewable: publiclyViewable,
		Validator:        validator,
	}
}

func (wrappa *WebAuthWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	wrapped := rata.Handlers{}

	loginPath, err := web.Routes.CreatePathForRoute(web.LogIn, nil)
	if err != nil {
		panic("could not construct login route")
	}

	loginRedirectRejector := auth.RedirectRejector{
		Location: loginPath,
	}

	for name, handler := range handlers {
		newHandler := handler

		if name != web.LogIn && !wrappa.PubliclyViewable {
			newHandler = auth.WrapHandler(
				handler,
				wrappa.Validator,
				loginRedirectRejector,
			)
		} else {
			newHandler = auth.WrapHandler(
				handler,
				auth.NoopValidator{},
				auth.UnauthorizedRejector{},
			)
		}

		switch name {
		case web.Index:
		case web.Pipeline:
		case web.TriggerBuild:
			newHandler = auth.WrapHandler(
				handler,
				wrappa.Validator,
				loginRedirectRejector,
			)
		case web.GetBuild:
		case web.GetBuilds:
		case web.GetJoblessBuild:
		case web.Public:
		case web.GetResource:
		case web.GetJob:
		case web.LogIn:
		case web.BasicAuth:
			newHandler = auth.WrapHandler(
				handler,
				wrappa.Validator,
				auth.BasicAuthRejector{},
			)
		case web.Debug:
			newHandler = auth.WrapHandler(
				handler,
				wrappa.Validator,
				loginRedirectRejector,
			)
		default:
			panic("you missed a spot")
		}

		wrapped[name] = newHandler
	}

	return wrapped
}
