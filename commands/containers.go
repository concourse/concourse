package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/fly/rc"
	"github.com/fatih/color"
	"github.com/tedsuo/rata"
)

type ContainersCommand struct{}

var containersCommand ContainersCommand

func init() {
	containers, err := Parser.AddCommand(
		"containers",
		"Print the running containers",
		"",
		&containersCommand,
	)
	if err != nil {
		panic(err)
	}

	containers.Aliases = []string{"cs"}
}

func (command *ContainersCommand) Execute([]string) error {
	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	atcRequester := newAtcRequester(target.URL(), target.Insecure)

	request, err := atcRequester.CreateRequest(atc.ListContainers, rata.Params{}, nil)
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

	var containers []atc.Container
	err = json.NewDecoder(response.Body).Decode(&containers)
	if err != nil {
		return err
	}

	headers := TableRow{
		{Contents: "handle", Color: color.New(color.Bold)},
		{Contents: "name", Color: color.New(color.Bold)},
		{Contents: "pipeline", Color: color.New(color.Bold)},
		{Contents: "type", Color: color.New(color.Bold)},
		{Contents: "build id", Color: color.New(color.Bold)},
		{Contents: "worker", Color: color.New(color.Bold)},
	}

	table := Table{headers}

	sort.Sort(byHandle(containers))

	for _, c := range containers {
		row := TableRow{
			{Contents: c.ID},
			{Contents: c.Name},
			{Contents: c.PipelineName},
			{Contents: c.Type},
			buildIDOrNone(c.BuildID),
			{Contents: c.WorkerName},
		}

		table = append(table, row)
	}

	fmt.Print(table.Render())

	return nil
}

type byHandle []atc.Container

func (cs byHandle) Len() int               { return len(cs) }
func (cs byHandle) Swap(i int, j int)      { cs[i], cs[j] = cs[j], cs[i] }
func (cs byHandle) Less(i int, j int) bool { return cs[i].ID < cs[j].ID }

func buildIDOrNone(id int) TableCell {
	var column TableCell

	if id == 0 {
		column.Contents = "none"
		column.Color = color.New(color.Faint)
	} else {
		column.Contents = strconv.Itoa(id)
	}

	return column
}
