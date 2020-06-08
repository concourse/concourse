package flaghelpers

import (
	"os"

	"github.com/jessevdk/go-flags"

	"github.com/concourse/concourse/fly/rc"
)

type flyCommand struct {
	Target rc.TargetName `short:"t" long:"target" description:"Concourse target name"`
}

func parseFlags() flyCommand {
	// Prevent go-flags from recursing
	goFlagsCompletion, hasCompletion := os.LookupEnv("GO_FLAGS_COMPLETION")
	os.Unsetenv("GO_FLAGS_COMPLETION")
	defer func() {
		if hasCompletion {
			os.Setenv("GO_FLAGS_COMPLETION", goFlagsCompletion)
		}
	}()

	var fly flyCommand
	parser := flags.NewParser(&fly, flags.HelpFlag|flags.PassDoubleDash|flags.IgnoreUnknown)
	parser.NamespaceDelimiter = "-"
	parser.Parse()

	return fly
}
