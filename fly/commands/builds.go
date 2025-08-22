package commands

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/fatih/color"
)

const timeDateLayout = "2006-01-02@15:04:05-0700"
const inputTimeLayout = "2006-01-02 15:04:05"

type BuildsCommand struct {
	AllTeams    bool                      `short:"a" long:"all-teams" description:"Show builds for the all teams that user has access to"`
	Count       int                       `short:"c" long:"count" default:"50" description:"Number of builds you want to limit the return to"`
	CurrentTeam bool                      `long:"current-team" description:"Show builds for the currently targeted team"`
	Job         flaghelpers.JobFlag       `short:"j" long:"job" value-name:"PIPELINE/JOB" description:"Name of a job to get builds for"`
	Json        bool                      `long:"json" description:"Print command result as JSON"`
	Pipeline    *flaghelpers.PipelineFlag `short:"p" long:"pipeline" description:"Name of a pipeline to get builds for"`
	Teams       []string                  `short:"n"  long:"team" description:"Show builds for these teams"`
	Since       string                    `long:"since" description:"Start of the range to filter builds. Expected time format of 'yyyy-mm-dd HH:mm:ss'"`
	Until       string                    `long:"until" description:"End of the range to filter builds. Expected time format of 'yyyy-mm-dd HH:mm:ss'"`
}

func (command *BuildsCommand) Execute([]string) error {
	var (
		builds = make([]atc.Build, 0)
		teams  = make([]concourse.Team, 0)

		timeSince time.Time
		timeUntil time.Time
		page      = concourse.Page{}
	)

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	page, err = command.validateBuildArguments(timeSince, page, timeUntil)
	if err != nil {
		return err
	}

	page.Limit = command.Count
	page.Timestamps = command.Since != "" || command.Until != ""

	currentTeam := target.Team()
	client := target.Client()

	builds, err = command.getBuilds(builds, currentTeam, page, client, teams)
	if err != nil {
		return err
	}

	return command.displayBuilds(builds)
}

func (command *BuildsCommand) getBuilds(builds []atc.Build, currentTeam concourse.Team, page concourse.Page, client concourse.Client, teams []concourse.Team) ([]atc.Build, error) {
	var err error

	if command.AllTeams {
		teams, err = command.getAllTeams(client, teams)
		if err != nil {
			return nil, err
		}
	} else if len(command.Teams) > 0 || command.CurrentTeam {
		teams = command.validateCurrentTeam(teams, currentTeam, client)
	}

	if len(teams) > 0 {
		for _, team := range teams {
			var teamBuilds []atc.Build
			if command.pipelineFlag() {
				teamBuilds, err = command.validatePipelineBuilds(builds, team, page)
				if err != nil {
					return nil, err
				}
			} else if command.jobFlag() {
				teamBuilds, err = command.validateJobBuilds(builds, team, page)
				if err != nil {
					return nil, err
				}
			} else {
				teamBuilds, _, err = team.Builds(page)
				if err != nil {
					return nil, err
				}

			}

			builds = append(builds, teamBuilds...)
		}
	} else {
		if command.pipelineFlag() {
			builds, err = command.validatePipelineBuilds(builds, currentTeam, page)
			if err != nil {
				return nil, err
			}
		} else if command.jobFlag() {
			builds, err = command.validateJobBuilds(builds, currentTeam, page)
			if err != nil {
				return nil, err
			}
		} else {
			builds, _, err = client.Builds(page)
			if err != nil {
				return nil, err
			}
		}

	}

	return builds, err
}

func (command *BuildsCommand) getAllTeams(client concourse.Client, teams []concourse.Team) ([]concourse.Team, error) {
	atcTeams, err := client.ListTeams()
	if err != nil {
		return nil, err
	}
	for _, atcTeam := range atcTeams {
		teams = append(teams, client.Team(atcTeam.Name))
	}
	return teams, nil
}

func (command *BuildsCommand) validateCurrentTeam(teams []concourse.Team, currentTeam concourse.Team, client concourse.Client) []concourse.Team {
	if command.CurrentTeam {
		teams = append(teams, currentTeam)
	}
	for _, teamName := range command.Teams {
		teams = append(teams, client.Team(teamName))
	}
	return teams
}

func (command *BuildsCommand) validateJobBuilds(builds []atc.Build, currentTeam concourse.Team, page concourse.Page) ([]atc.Build, error) {
	var (
		err   error
		found bool
	)

	builds, _, found, err = currentTeam.JobBuilds(
		command.Job.PipelineRef,
		command.Job.JobName,
		page,
	)
	if err != nil {
		return nil, err
	}
	if !found {
		displayhelpers.Failf("pipeline/job not found")
	}
	return builds, err
}

func (command *BuildsCommand) validatePipelineBuilds(builds []atc.Build, currentTeam concourse.Team, page concourse.Page) ([]atc.Build, error) {
	err := command.Pipeline.Validate()
	if err != nil {
		return nil, err
	}

	var found bool
	builds, _, found, err = currentTeam.PipelineBuilds(
		command.Pipeline.Ref(),
		page,
	)

	if err != nil {
		return nil, err
	}

	if !found {
		displayhelpers.Failf("pipeline not found")
	}

	return builds, err
}

func (command *BuildsCommand) displayBuilds(builds []atc.Build) error {
	var err error
	if command.Json {
		err = displayhelpers.JsonPrint(builds)
		if err != nil {
			return err
		}
		return nil
	}

	table := ui.Table{
		Headers: ui.TableRow{
			{Contents: "id", Color: color.New(color.Bold)},
			{Contents: "name", Color: color.New(color.Bold)},
			{Contents: "status", Color: color.New(color.Bold)},
			{Contents: "start", Color: color.New(color.Bold)},
			{Contents: "end", Color: color.New(color.Bold)},
			{Contents: "duration", Color: color.New(color.Bold)},
			{Contents: "team", Color: color.New(color.Bold)},
			{Contents: "created by", Color: color.New(color.Bold)},
		},
	}

	buildCap := command.buildCap(builds)
	for _, b := range builds[:buildCap] {
		startTimeCell, endTimeCell, durationCell := populateTimeCells(time.Unix(b.StartTime, 0), time.Unix(b.EndTime, 0))

		var nameCell ui.TableCell

		var names []string
		if b.PipelineName != "" {
			pipelineRef := atc.PipelineRef{
				Name:         b.PipelineName,
				InstanceVars: b.PipelineInstanceVars,
			}

			names = append(names, pipelineRef.String())
		}

		if b.JobName != "" {
			names = append(names, b.JobName)
		}

		if b.ResourceName != "" {
			names = append(names, b.ResourceName)
		}

		names = append(names, b.Name)

		nameCell.Contents = strings.Join(names, "/")

		createdBy := "system"
		if b.CreatedBy != nil {
			createdBy = *b.CreatedBy
		}
		table.Data = append(table.Data, []ui.TableCell{
			{Contents: strconv.Itoa(b.ID)},
			nameCell,
			ui.BuildStatusCell(b.Status),
			startTimeCell,
			endTimeCell,
			durationCell,
			{Contents: b.TeamName},
			{Contents: createdBy},
		})
	}

	return table.Render(os.Stdout, Fly.PrintTableHeaders)
}

func (command *BuildsCommand) validateBuildArguments(timeSince time.Time, page concourse.Page, timeUntil time.Time) (concourse.Page, error) {
	var err error
	if command.Since != "" {
		timeSince, err = time.ParseInLocation(inputTimeLayout, command.Since, time.Now().Location())
		if err != nil {
			return page, errors.New("Since time should be in the format: " + inputTimeLayout)
		}
		page.From = int(timeSince.Unix())
	}
	if command.Until != "" {
		timeUntil, err = time.ParseInLocation(inputTimeLayout, command.Until, time.Now().Location())
		if err != nil {
			return page, errors.New("Until time should be in the format: " + inputTimeLayout)
		}
		page.To = int(timeUntil.Unix())
	}
	if timeSince.After(timeUntil) && command.Since != "" && command.Until != "" {
		return page, errors.New("Cannot have --since after --until")
	}
	if command.pipelineFlag() && command.jobFlag() {
		return page, errors.New("Cannot specify both --pipeline and --job")
	}
	if command.CurrentTeam && command.AllTeams {
		return page, errors.New("Cannot specify both --all-teams and --current-team")
	}
	if len(command.Teams) > 0 && command.AllTeams {
		return page, errors.New("Cannot specify both --all-teams and --team")
	}
	return page, err
}

func populateTimeCells(startTime time.Time, endTime time.Time) (ui.TableCell, ui.TableCell, ui.TableCell) {
	var startTimeCell ui.TableCell
	var endTimeCell ui.TableCell
	var durationCell ui.TableCell

	startTime = startTime.Local()
	endTime = endTime.Local()
	zeroTime := time.Unix(0, 0)

	if startTime == zeroTime {
		startTimeCell.Contents = "n/a"
	} else {
		startTimeCell.Contents = startTime.Format(timeDateLayout)
	}

	if endTime == zeroTime {
		endTimeCell.Contents = "n/a"
		durationCell.Contents = fmt.Sprintf("%v+", roundSecondsOffDuration(time.Since(startTime)))
	} else {
		endTimeCell.Contents = endTime.Format(timeDateLayout)
		durationCell.Contents = endTime.Sub(startTime).String()
	}

	if startTime == zeroTime {
		durationCell.Contents = "n/a"
	}

	return startTimeCell, endTimeCell, durationCell
}

func roundSecondsOffDuration(d time.Duration) time.Duration {
	return d - (d % time.Second)
}

func (command *BuildsCommand) jobFlag() bool {
	return command.Job.PipelineRef.Name != "" && command.Job.JobName != ""
}

func (command *BuildsCommand) pipelineFlag() bool {
	return command.Pipeline != nil
}

func (command *BuildsCommand) buildCap(builds []atc.Build) int {
	if command.Count < len(builds) {
		return command.Count
	}

	return len(builds)
}
