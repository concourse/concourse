package atc

type Job struct {
	NextBuild     *Build `json:"next_build"`
	FinishedBuild *Build `json:"finished_build"`
}
