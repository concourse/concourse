package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/fatih/color"
	"github.com/tedsuo/rata"
)

type PipelinesCommand struct{}

var pipelinesCommand PipelinesCommand

func init() {
	command, err := Parser.AddCommand(
		"pipelines",
		"Print the configured pipelines",
		"",
		&pipelinesCommand,
	)
	if err != nil {
		panic(err)
	}

	command.Aliases = []string{"p"}
}

func (command *PipelinesCommand) Execute([]string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	atcRequester := newAtcRequester(target.URL(), target.Insecure)

	request, err := atcRequester.CreateRequest(atc.ListPipelines, rata.Params{}, nil)
	if err != nil {
		return err
	}

	response, err := atcRequester.httpClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode == http.StatusInternalServerError {
		return errors.New("unexpected server error")
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response code: %s", response.Status)
	}

	var pipelines []atc.Pipeline
	err = json.NewDecoder(response.Body).Decode(&pipelines)
	if err != nil {
		return err
	}

	table := Table{
		{{Contents: "name", Color: color.New(color.Bold)}, {Contents: "paused", Color: color.New(color.Bold)}},
	}

	for _, p := range pipelines {
		var pausedColumn TableCell
		if p.Paused {
			pausedColumn.Contents = "yes"
			pausedColumn.Color = color.New(color.FgCyan)
		} else {
			pausedColumn.Contents = "no"
		}

		table = append(table, []TableCell{
			{Contents: p.Name},
			pausedColumn,
		})
	}

	fmt.Print(table.Render())

	return nil
}
