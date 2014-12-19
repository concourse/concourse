package atc

type Build struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	JobName string `json:"job_name"`
	URL     string `json:"url"`
}

type BuildStatus string

const (
	StatusStarted   BuildStatus = "started"
	StatusSucceeded BuildStatus = "succeeded"
	StatusFailed    BuildStatus = "failed"
	StatusErrored   BuildStatus = "errored"
	StatusAborted   BuildStatus = "aborted"
)

type BuildPlan struct {
	Privileged bool `json:"privileged"`

	Config BuildConfig `json:"config"`

	Inputs  []InputPlan  `json:"inputs"`
	Outputs []OutputPlan `json:"outputs"`
}

type BuildConfig struct {
	Image  string             `json:"image,omitempty"   yaml:"image"`
	Params map[string]string  `json:"params,omitempty"  yaml:"params"`
	Run    BuildRunConfig     `json:"run,omitempty"     yaml:"run"`
	Inputs []BuildInputConfig `json:"inputs,omitempty"  yaml:"inputs"`
}

type BuildRunConfig struct {
	Path string   `json:"path" yaml:"path"`
	Args []string `json:"args,omitempty" yaml:"args"`
}

type BuildInputConfig struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path,omitempty" yaml:"path"`
}

type InputPlan struct {
	// logical name of the input with respect to the build's config
	Name string `json:"name"`

	// name of resource providing the input
	Resource string `json:"resource"`

	// type of resource
	Type string `json:"type"`

	// e.g. sha
	Version Version `json:"version,omitempty"`

	// e.g. git url, branch, private_key
	Source Source `json:"source,omitempty"`

	// arbitrary config for input
	Params Params `json:"params,omitempty"`

	// path to build configuration provided by this input
	ConfigPath string `json:"config_path"`
}

type OutputPlan struct {
	Name string `json:"name"`

	Type string `json:"type"`

	// e.g. [success, failure]
	On OutputConditions `json:"on,omitempty"`

	// e.g. git url, branch, private_key
	Source Source `json:"source,omitempty"`

	// arbitrary config for output
	Params Params `json:"params,omitempty"`
}

type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type OutputConditions []OutputCondition

func (cs OutputConditions) SatisfiedBy(exitStatus int) bool {
	for _, status := range cs {
		if (status == OutputConditionSuccess && exitStatus == 0) ||
			(status == OutputConditionFailure && exitStatus != 0) {
			return true
		}
	}

	return false
}
