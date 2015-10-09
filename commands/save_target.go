package commands

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type SaveTargetCommand struct {
	API      string   `long:"api" required:"true"           description:"Api url to target"`
	Username string   `long:"username"                      description:"Username for the api"`
	Password string   `long:"password"                      description:"Password for the api"`
	Cert     PathFlag `long:"cert"                          description:"directory to your cert"`
	Name     string   `short:"n" long:"name" required:"true" description:"Name for target"`
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

type targetProps struct {
	API      string `yaml:"api"`
	Username string
	Password string
	Cert     string
}

type TargetDetailsYAML struct {
	Targets map[string]targetProps
}

func (command *SaveTargetCommand) Execute(args []string) error {
	flyrc := filepath.Join(userHomeDir(), ".flyrc")

	targetName := command.Name
	if targetName == "" {
		log.Fatalln("name not provided for target")
		return nil
	}

	if _, err := os.Stat(flyrc); err != nil {
		createTargets(flyrc, command, targetName)
	} else {
		updateTargets(flyrc, command, targetName)
	}

	fmt.Printf("successfully saved target %s\n", targetName)
	return nil
}

func createTargets(location string, command *SaveTargetCommand, targetName string) {
	targetsBytes, err := yaml.Marshal(&TargetDetailsYAML{
		Targets: map[string]targetProps{
			targetName: {
				API:      command.API,
				Username: command.Username,
				Password: command.Password,
				Cert:     string(command.Cert),
			},
		},
	})
	if err != nil {
		log.Fatalln("could not marshal YAML")
		return
	}

	err = ioutil.WriteFile(location, targetsBytes, os.ModePerm)
	if err != nil {
		log.Fatalln("could not create .flyrc")
	}
}

func updateTargets(location string, command *SaveTargetCommand, targetToUpdate string) {
	yamlToSet := targetProps{
		API:      command.API,
		Username: command.Username,
		Password: command.Password,
		Cert:     string(command.Cert),
	}

	currentTargetsBytes, err := ioutil.ReadFile(location)
	if err != nil {
		log.Fatalln("could not read .flyrc")
		return
	}

	var current *TargetDetailsYAML
	err = yaml.Unmarshal(currentTargetsBytes, &current)
	if err != nil {
		log.Fatalln("could not unmarshal .flyrc")
		return
	}

	current.Targets[targetToUpdate] = yamlToSet

	yamlBytes, err := yaml.Marshal(current)
	if err != nil {
		log.Fatalln("could not marshal .flyrc yaml")
		return
	}

	err = ioutil.WriteFile(location, yamlBytes, os.ModePerm)
	if err != nil {
		log.Fatalln("could not write .flyrc")
		return
	}
}
