package commands

import (
	"fmt"
	"github.com/concourse/concourse/fly/rc"
	"io/ioutil"
	"net/http/httputil"
	"os"
)

type CurlCommand struct {
	Method        string   `short:"X" description:"method" default:"GET"`
	OutputHeaders bool     `short:"I" description:"output response headers"`
	Headers       []string `short:"H" description:"HTTP Header"`
	Body          string   `short:"d" description:"Body"`
}

func (command *CurlCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("path required")
	}

	cr := CurlRequest{
		Host:    target.URL(),
		Path:    args[0],
		Method:  command.Method,
		Headers: command.Headers,
		Body:    command.Body,
	}

	req, err := cr.CreateHttpRequest()
	if err != nil {
		return err
	}

	res, err := target.Client().HTTPClient().Do(req)
	if err != nil {
		return err
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("%s\n%s", res.Status, resBody)
	}

	if command.OutputHeaders {
		headers, err := httputil.DumpResponse(res, false)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(headers))
		return nil
	}

	fmt.Fprintln(os.Stdout, string(resBody))
	return nil
}
