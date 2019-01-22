package commands

import (
	"fmt"
	"github.com/concourse/concourse/fly/rc"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type CurlCommand struct {
	Args struct {
		Path string   `positional-arg-name:"PATH" required:"true"`
		Rest []string `positional-arg-name:"curl flags"`
	} `positional-args:"yes"`
	PrintAndExit bool `long:"print-and-exit" description:"Print curl command and exit"`
}

func (command *CurlCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	if target.CACert() != "" {
		return fmt.Errorf("not implemented for custom CA Certs")
	}

	fullUrl, err := command.makeFullUrl(target.URL(), command.Args.Path)
	if err != nil {
		return err
	}

	argsList := command.makeArgsList(target.Token(), fullUrl, command.Args.Rest)

	cmd := exec.Command("curl", argsList...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if command.PrintAndExit {
		fmt.Println(printableCommand(cmd.Args))
		return nil
	}

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func (command *CurlCommand) makeFullUrl(host, path string) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", err
	}
	u.Path = path
	return u.String(), nil
}

func (command *CurlCommand) makeArgsList(token *rc.TargetToken, url string, options []string) (args []string) {
	authTokenHeader := []string{"-H", fmt.Sprintf("Authorization: %s %s", token.Type, token.Value)}
	args = append(args, authTokenHeader...)
	args = append(args, options...)
	args = append(args, url)
	return
}

func printableCommand(args []string) string {
	/*
	Annoyance. If we execute
		fly -t team curl /api/v1/teams/team/pipelines/foo  -- -H 'yay: hazzah'
	And we simply
		strings.join(args, " ")
	The single quotes are lost.

	We would like to see
		curl -H 'Authorization: Bearer token' -H 'yay: hazzah' url/api/v1/teams/team/pipelines/foo
	But instead it gives us
		curl -H Authorization: Bearer token -H yay: hazzah url/api/v1/teams/team/pipelines/foo

	Thus making the printableCommand non copy-pasteable.
	*/

	return strings.Join(args, " ")
}
