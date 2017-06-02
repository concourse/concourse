package plan

import "github.com/concourse/atc"

type Plan struct {
	Steps []interface{}
}

type CheckStep struct {
	Actions []interface{}
}

type GetStep struct {
	Actions []interface{}
}

type TaskStep struct {
	Actions []interface{}
}

type PutStep struct {
	Actions []interface{}
}

type CheckAction struct {
	RootFSSource interface{}
	Source       atc.Source
}

type GetAction struct {
	RootFSSource interface{}
	Version      atc.Version
	Params       atc.Params
	Source       atc.Source
	Outputs      []Output
}

type TaskAction struct {
	RootFSSource interface{}
	Platform     string
	Run          atc.TaskRunConfig
	Params       map[string]string
	Inputs       []Input
	Outputs      []Output
}

type PutAction struct {
	RootFSSource interface{}
	Params       map[string]string
	Inputs       []Input
}

type Output struct {
	Name string
	Path string
}

type Input struct {
	Name string
	Path string
}

type BaseResourceTypeRootFSSource struct {
	Name string
}

type OutputRootFSSource struct {
	Name string
}

type DynamicRootFSSource struct {
}

type LookupFileConfigurationStep struct {
}
