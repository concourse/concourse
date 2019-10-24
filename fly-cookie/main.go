package main

import (
	"bytes"
	"fmt"
	"github.com/antchfx/jsonquery"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/event"
	atc_event "github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/fly/commands"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/prometheus/common/log"
	"io"
	"regexp"
	"sync"
	"time"
)

type StepData struct {
	kind string
	name string
	id   string
}

type Output struct {
	team     string
	pipeline string
	job      string
	build    string
	step     StepData
	time     time.Time
	message  string
	failure  string
}

type Options struct {
}

var noVmwareWorkerFailure *regexp.Regexp
var noTaskFileFailure *regexp.Regexp

func init() {
	noVmwareWorkerFailure = regexp.MustCompile(`^no workers satisfying: .* tag 'vmware'$`)
	noTaskFileFailure = regexp.MustCompile(`^task config '.*' not found$`)
}

func main() {
	login := &commands.LoginCommand{BrowserOnly: true}
	err := login.Execute([]string{})

	target, err := rc.LoadTarget("raas-pks-infra", false)
	//target, err := rc.LoadTarget("infra", false)
	if err != nil {
		log.Fatal(err)
	}

	err = exec2(target)
	if err != nil {
		log.Fatal(err)
	}
}

func exec(target rc.Target) error {
	//var headers []string
	var jobs []atc.Job

	pipelines, err := target.Team().ListPipelines()
	if err != nil {
		return err
	}

	for _, pipeline := range pipelines {
		fmt.Println(pipeline.Name)

		jobs, err = target.Team().ListJobs(pipeline.Name)
		if err != nil {
			return err
		}

		for _, job := range jobs {
			fmt.Println("\t", job.Name)
			builds, _, _, err := target.Team().JobBuilds(pipeline.Name, job.Name, concourse.Page{})
			if err != nil {
				return err
			}

			for _, build := range builds {
				fmt.Println("\t\t", build.Name, build.Status)
			}
		}
	}

	return nil
}

func exec2(target rc.Target) error {

	team := target.Team()

	pipelines, err := target.Team().ListPipelines()
	if err != nil {
		return err
	}

	result := make(chan Output)
	wg := sync.WaitGroup{}

	for _, pipeline := range pipelines {
		wg.Add(1)
		go func() {
			//func() {
			defer wg.Done()
			err := listFailedSteps(target, pipeline, team, result)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	outputs := make([]Output, 0)
	failures := map[string]int{}

	go func() {
		for output := range result {
			outputs = append(outputs, output)

			if output.failure != "succeeded" {
				fmt.Println(output.team, output.pipeline, output.job, output.build, output.step.kind, output.step.name, output.time, output.message)
			}

			if _, ok := failures[output.failure]; !ok {
				failures[output.failure] = 0
			}
			failures[output.failure] += 1
		}
	}()

	wg.Wait()
	close(result)

	fmt.Println("\nFailures:")
	for failure, count := range failures {
		fmt.Println(failure, count)
	}

	return nil
}

func listFailedSteps(target rc.Target, pipeline atc.Pipeline, team concourse.Team, result chan<- Output) error {
	jobs, err := target.Team().ListJobs(pipeline.Name)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		builds, _, _, err := target.Team().JobBuilds(pipeline.Name, job.Name, concourse.Page{})
		if err != nil {
			return err
		}

		for _, build := range builds {
			if build.EndTime == 0 {
				continue
			}

			//if build.Status == "succeeded" {
			//    continue
			//}

			plan, _, err := target.Client().BuildPlan(build.ID)
			if err != nil {
				return err
			}

			steps, err := findStepsById(plan)
			if err != nil {
				return err
			}

			events, err := findFailureEvents(target, build)
			if err != nil {
				return err
			}

			for id, ev := range events {
				output := Output{
					team:     team.Name(),
					pipeline: pipeline.Name,
					job:      job.Name,
					build:    build.Name,
					step:     steps[id],
				}

				switch e := ev.(type) {
				case event.FinishTask:
					output.time = time.Unix(e.Time, 0)
					if e.ExitStatus == 0 {
						output.message = ""
						output.failure = "succeeded"
					} else {
						output.message = fmt.Sprintf("exit: %d", e.ExitStatus)
						output.failure = "step"
					}
				case event.Error:
					output.time = time.Unix(e.Time, 0)
					output.message = e.Message
					if e.Message == "timeout exceeded" {
						output.failure = "timeout"
					} else if e.Message == "interrupted" {
						output.failure = "cancelled"
					} else if noVmwareWorkerFailure.MatchString(e.Message) {
						output.failure = "no-worker-vmware"
					} else if noTaskFileFailure.MatchString(e.Message) {
						output.failure = "no-task-file"
					} else {
						output.failure = e.Message
					}
				}

				result <- output
			}
		}
	}
	return nil
}

func findStepsById(plan atc.PublicBuildPlan) (map[atc_event.OriginID]StepData, error) {
	b, err := plan.Plan.MarshalJSON()
	if err != nil {
		return nil, err
	}
	r := bytes.NewReader(b)
	doc, err := jsonquery.Parse(r)
	named_steps := jsonquery.Find(doc, "//*[id and *[name]]")

	steps := map[atc_event.OriginID]StepData{}

	for _, node := range named_steps {
		tree := jsonquery.FindOne(node, "//*[name]").Parent
		id := jsonquery.FindOne(tree, "/id").FirstChild.Data
		nameNode := jsonquery.FindOne(tree, "/*/name")

		steps[atc_event.OriginID(id)] = StepData{
			id:   id,
			kind: nameNode.Parent.Data,
			name: nameNode.FirstChild.Data,
		}
	}

	return steps, nil
}

func findFailureEvents(target rc.Target, build atc.Build) (map[atc_event.OriginID]atc.Event, error) {
	src, err := target.Client().BuildEvents(fmt.Sprintf("%d", build.ID))
	defer src.Close()
	if err != nil {
		return nil, err
	}

	events := map[atc_event.OriginID]atc.Event{}

	for {
		ev, err := src.NextEvent()
		if err != nil {
			if err == io.EOF {
				return events, nil
			} else {
				return events, err
			}
		}

		switch e := ev.(type) {
		case event.Log:
		case event.InitializeTask:
		case event.StartTask:
		case event.FinishTask:
			events[e.Origin.ID] = e
		case event.Error:
			events[e.Origin.ID] = e
		case event.Status:
			continue
		}
	}
}
