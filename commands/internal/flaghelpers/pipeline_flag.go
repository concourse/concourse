package flaghelpers

import (
	"os"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/concourse/fly/rc"
)

type flyCommand struct {
	Target rc.TargetName `short:"t" long:"target" description:"Concourse target name"`
}

var fly flyCommand

type PipelineFlag string

func (flag *PipelineFlag) Complete(match string) []flags.Completion {
	// Prevent go-flags from recursing
	goFlagsCompletion, hasCompletion := os.LookupEnv("GO_FLAGS_COMPLETION")
	os.Unsetenv("GO_FLAGS_COMPLETION")
	defer func() {
		if hasCompletion {
			os.Setenv("GO_FLAGS_COMPLETION", goFlagsCompletion)
		}
	}()

	parser := flags.NewParser(&fly, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"
	parser.Parse()

	target, err := rc.LoadTarget(fly.Target)
	if err != nil {
		return []flags.Completion{}
	}

	err = target.Validate()
	if err != nil {
		return []flags.Completion{}
	}

	pipelines, err := target.Team().ListPipelines()
	if err != nil {
		return []flags.Completion{}
	}

	comps := []flags.Completion{}
	for _, pipeline := range pipelines {
		if strings.HasPrefix(pipeline.Name, match) {
			comps = append(comps, flags.Completion{Item: pipeline.Name})
		}
	}

	return comps
}
