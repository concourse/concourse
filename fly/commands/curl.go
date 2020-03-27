package commands

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/concourse/concourse/fly/rc"
)

type CurlCommand struct {
	Args struct {
		Path string   `positional-arg-name:"PATH" required:"true" description:"Pass query params as normal curl path like path?key=value"`
		Rest []string `positional-arg-name:"curl flags" description:"To pass flags to curl, pass a -- argument, so that fly can distinguish them from its own flags"`
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
	p, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	u.Path = p.Path
	u.RawQuery = p.RawQuery
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
	for i, arg := range args {
		if strings.Contains(arg, " ") {
			args[i] = fmt.Sprintf(`"%s"`, arg)
		}
	}

	return strings.Join(args, " ")
}
