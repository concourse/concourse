package commands

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/codegangsta/cli"
	"gopkg.in/yaml.v2"
)

type targetsYAML struct {
	Targets map[string]yaml.MapSlice
}

func SaveTarget(c *cli.Context) {
	flyrc := filepath.Join(userHomeDir(), ".flyrc")

	if c.Args().First() == "" {
		log.Fatalln("name not provided for target")
		return
	}

	if _, err := os.Stat(flyrc); err != nil {
		createTargets(flyrc, c)
	} else {
		updateTargets(flyrc, c)
	}

	fmt.Printf("successfully saved target %s", c.Args().First())
}

func createTargets(location string, c *cli.Context) {
	targetName := c.Args().First()

	targetsBytes, err := yaml.Marshal(&targetsYAML{
		Targets: map[string]yaml.MapSlice{
			targetName: {
				{Key: "api", Value: c.String("api")},
				{Key: "username", Value: c.String("username")},
				{Key: "password", Value: c.String("password")},
				{Key: "cert", Value: c.String("cert")},
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

func updateTargets(location string, c *cli.Context) {
	targetToUpdate := c.Args().First()
	yamlToSet := yaml.MapSlice{
		{Key: "api", Value: c.String("api")},
		{Key: "username", Value: c.String("username")},
		{Key: "password", Value: c.String("password")},
		{Key: "cert", Value: c.String("cert")},
	}

	currentTargetsBytes, err := ioutil.ReadFile(location)
	if err != nil {
		log.Fatalln("could not read .flyrc")
		return
	}

	var current *targetsYAML
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
