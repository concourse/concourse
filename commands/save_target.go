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

type targetProps struct {
	API      string `yaml:"api"`
	Username string
	Password string
	Cert     string
}

type TargetDetailsYAML struct {
	Targets map[string]targetProps
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

	fmt.Printf("successfully saved target %s\n", c.Args().First())
}

func createTargets(location string, c *cli.Context) {
	targetName := c.Args().First()

	targetsBytes, err := yaml.Marshal(&TargetDetailsYAML{
		Targets: map[string]targetProps{
			targetName: {
				API:      c.String("api"),
				Username: c.String("username"),
				Password: c.String("password"),
				Cert:     c.String("cert"),
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
	yamlToSet := targetProps{
		API:      c.String("api"),
		Username: c.String("username"),
		Password: c.String("password"),
		Cert:     c.String("cert"),
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
