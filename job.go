package atc

type Job struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	NextBuild     *Build `json:"next_build"`
	FinishedBuild *Build `json:"finished_build"`
}
