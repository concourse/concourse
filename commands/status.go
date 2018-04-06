package commands

import (
	"errors"
	"fmt"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	jwt "github.com/dgrijalva/jwt-go"
)

type StatusCommand struct{}

func (c *StatusCommand) Execute([]string) error {
	if Fly.Target == "" {
		return errors.New("name for the target must be specified (--target/-t)")
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		displayhelpers.FailWithErrorf("Please login again. ", err)
	}

	tToken := target.Token()

	if tToken != nil && tToken.Value != "" {
		_, err := jwt.Parse(tToken.Value, func(token *jwt.Token) (interface{}, error) {
			return nil, token.Claims.Valid()
		})

		if err != nil && err.Error() != jwt.ErrInvalidKeyType.Error() {
			displayhelpers.FailWithErrorf("Please login again.\n\nToken Validation failed with error ", err)
			return nil
		}
	}

	fmt.Println("Loogged in successfully!")
	return nil
}
