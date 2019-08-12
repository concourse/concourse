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
		Path  string   `positional-arg-name:"PATH" required:"true"`
		Query string   `positional-arg-name:"QUERY"`
		Rest  []string `positional-arg-name:"curl flags"`
	} `positional-args:"yes"`
	PrintAndExit bool `long:"print-and-exit" description:"Print curl command and exit"`
}

func (command *CurlCommand) Execute([]string) error {
	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	fullUrl, err := command.makeFullUrl(target.URL(), command.Args.Path, command.Args.Query)
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

func (command *CurlCommand) makeFullUrl(host, path string, query string) (string, error) {
	u, err := url.Parse(host)
	if err != nil {
		return "", err
	}
	u.Path = path
	u.RawQuery = query
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
