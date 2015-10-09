package commands

import (
	"strings"

	"github.com/concourse/atc"
)

type JobFlag struct {
	PipelineName string
	JobName      string
}

func (job *JobFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "/", 2)
	if len(vs) != 2 {
		job.PipelineName = atc.DefaultPipelineName
		job.JobName = vs[0]
	} else {
		job.PipelineName = vs[0]
		job.JobName = vs[1]
	}

	return nil
}
