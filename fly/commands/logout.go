package commands

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/fly/rc"
)

type LogoutCommand struct {
	All bool `short:"a" long:"all" description:"Logout of all targets"`
}

func (command *LogoutCommand) Execute(args []string) error {
	if Fly.Target != "" && !command.All {
		return command.logoutSingleTarget(Fly.Target)
	} else if Fly.Target == "" && command.All {

		targets, err := rc.LoadTargets()
		if err != nil {
			return err
		}

		errs := []error{}
		for targetName := range targets {
			if err := command.logoutSingleTarget(targetName); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", string(targetName), err))
			}
		}

		if len(errs) > 0 {
			return errors.Join(errs...)
		}

		fmt.Println("logged out of all targets")
	} else {
		return errors.New("must specify either --target or --all")
	}

	return nil
}

func (cmd *LogoutCommand) logoutSingleTarget(targetName rc.TargetName) error {
	target, err := rc.LoadTarget(targetName, Fly.Verbose)
	if err != nil {
		return fmt.Errorf("failed to load target %q: %w", targetName, err)
	}

	targetToken := target.Token()
	if targetToken != nil && targetToken.Type != "" && targetToken.Value != "" {
		if err := cmd.logoutAPI(target.URL(), targetToken); err != nil {
			return fmt.Errorf("failed to logout from API: %w", err)
		}
	}

	if err := rc.LogoutTarget(targetName); err != nil {
		return fmt.Errorf("failed to remove target %q: %w", targetName, err)
	}

	fmt.Printf("logged out of target: %s\n", targetName)
	return nil
}

func (cmd *LogoutCommand) logoutAPI(apiURL string, targetToken *rc.TargetToken) error {
	req, err := http.NewRequest("GET", apiURL+"/sky/logout", nil)
	if err != nil {
		return fmt.Errorf("failed to create logout request: %w", err)
	}

	req.AddCookie(&http.Cookie{
		Name:  "skymarshal_auth",
		Value: targetToken.Type + " " + targetToken.Value,
	})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("logout request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("logout failed with status: %s", resp.Status)
	}

	return nil
}
