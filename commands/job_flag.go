package commands

import (
	"fmt"
	"strings"
)

type JobFlag struct {
	PipelineName string
	JobName      string
}

func (job *JobFlag) UnmarshalFlag(value string) error {
	vs := strings.SplitN(value, "/", 2)
	if len(vs) != 2 {
		return fmt.Errorf("invalid job '%s' (must be pipeline-name/job-name)", value)
	}

	job.PipelineName = vs[0]
	job.JobName = vs[1]

	return nil
}
