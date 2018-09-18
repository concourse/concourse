package flaghelpers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/go-concourse/concourse"
	"github.com/jessevdk/go-flags"

	"github.com/concourse/fly/rc"
)

type JobFlag struct {
	PipelineName string
	JobName      string
}

func (job *JobFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "/", -1)

	if len(vs) != 2 {
		return errors.New("argument format should be <pipeline>/<job>")
	}

	if vs[0] == "" {
		return concourse.NameRequiredError("pipeline")
	}

	if vs[1] == "" {
		return concourse.NameRequiredError("job")
	}

	job.PipelineName = vs[0]
	job.JobName = vs[1]

	return nil
}

func (flag *JobFlag) Complete(match string) []flags.Completion {
	fly := parseFlags()

	target, err := rc.LoadTarget(fly.Target, false)
	if err != nil {
		return []flags.Completion{}
	}

	err = target.Validate()
	if err != nil {
		return []flags.Completion{}
	}

	team := target.Team()
	comps := []flags.Completion{}
	vs := strings.SplitN(match, "/", 2)

	if len(vs) == 1 {
		pipelines, err := team.ListPipelines()
		if err != nil {
			return comps
		}

		for _, pipeline := range pipelines {
			if strings.HasPrefix(pipeline.Name, vs[0]) {
				comps = append(comps, flags.Completion{Item: pipeline.Name + "/"})
			}
		}
	} else if len(vs) == 2 {
		jobs, err := team.ListJobs(vs[0])
		if err != nil {
			return comps
		}

		for _, job := range jobs {
			if strings.HasPrefix(job.Name, vs[1]) {
				comps = append(comps, flags.Completion{Item: fmt.Sprintf("%s/%s", vs[0], job.Name)})
			}
		}
	}

	return comps
}
