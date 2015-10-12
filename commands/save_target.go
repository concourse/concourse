package commands

import (
	"fmt"
	"log"

	"github.com/concourse/fly/rc"
)

type SaveTargetCommand struct {
	API      string   `long:"api" required:"true"            description:"Api url to target"`
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
	targetName := command.Name
	if targetName == "" {
		log.Fatalln("name not provided for target")
		return nil
	}

	err := rc.CreateOrUpdateTargets(
		targetName,
		rc.NewTarget(
			command.API,
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
