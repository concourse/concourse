package commands

import (
	"fmt"
	"github.com/concourse/concourse/fly/commands/internal/curler"
	"github.com/concourse/concourse/fly/rc"
	"github.com/pkg/errors"
	"strings"
)

type CurlCommand struct {
}

func (command *CurlCommand) validate(args []string) error {
	if len(args) != 1 {
		return errors.New("please provide a path you wish to curl")
	}

	if !strings.HasPrefix(args[0], "/api") {
		return errors.New("path must start with /api")
	}
	return nil
}

func (command *CurlCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	if err := target.Validate(); err != nil {
		return err
	}

	if err := command.validate(args); err != nil {
		return err
	}

	curl := curler.New(target.Client().HTTPClient(), target.URL())

	body, respHeaders, err := curl.It(args[0])
	if err != nil {
		return err
	}

	fmt.Println(string(respHeaders))
	fmt.Println(string(body))


	return nil
}
