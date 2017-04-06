package atc

import (
	"errors"

	multierror "github.com/hashicorp/go-multierror"
)

type AuthFlags struct {
	NoAuth bool `long:"no-really-i-dont-want-any-auth" description:"Ignore warnings about not configuring auth"`

	BasicAuth BasicAuthFlag `group:"Basic Authentication" namespace:"basic-auth"`
}

type BasicAuthFlag struct {
	Username string `long:"username" description:"Username to use for basic auth."`
	Password string `long:"password" description:"Password to use for basic auth."`
}

func (auth *BasicAuthFlag) IsConfigured() bool {
	return auth.Username != "" || auth.Password != ""
}

func (auth *BasicAuthFlag) Validate() error {
	var errs *multierror.Error
	if auth.Username == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --basic-auth-username to use basic auth."),
		)
	}
	if auth.Password == "" {
		errs = multierror.Append(
			errs,
			errors.New("must specify --basic-auth-password to use basic auth."),
		)
	}
	return errs.ErrorOrNil()
}
