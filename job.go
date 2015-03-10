package atc

type Job struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	NextBuild     *Build `json:"next_build"`
	FinishedBuild *Build `json:"finished_build"`

	Inputs  []JobInput  `json:"inputs"`
	Outputs []JobOutput `json:"outputs"`

	Groups []string `json:"groups"`
}

type JobInput struct {
	Name     string   `json:"name,omitempty"`
	Resource string   `json:"resource"`
	Passed   []string `json:"passed,omitempty"`
	Trigger  bool     `json:"trigger"`
}

type JobOutput struct {
	Resource  string      `json:"resource"`
	PerformOn []Condition `json:"perform_on"`
}
