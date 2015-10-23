package commands

import (
	"fmt"
	"log"

	"github.com/concourse/fly/rc"
)

type SaveTargetCommand struct {
	Username string   `long:"username"                       description:"Username for the api"`
	Password string   `long:"password"                       description:"Password for the api"`
	Cert     PathFlag `long:"cert"                           description:"Directory to your cert"`
	Name     string   `short:"n" long:"name" required:"true" description:"Name for target"`
	Insecure bool     `long:"skip-ssl"                       description:"Skip SSL verification"`
}

var saveTargetCommand SaveTargetCommand

func init() {
	_, err := Parser.AddCommand(
		"save-target",
		"Save a fly target to the .flyrc",
		"",
		&saveTargetCommand,
	)
	if err != nil {
		panic(err)
	}
}

func (command *SaveTargetCommand) Execute(args []string) error {
	targetAPI := globalOptions.Target
	targetName := command.Name

	err := rc.CreateOrUpdateTargets(
		targetName,
		rc.NewTarget(
			targetAPI,
			command.Username,
			command.Password,
			string(command.Cert),
			command.Insecure,
		),
	)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	fmt.Printf("successfully saved target %s\n", targetName)
	return nil
}
