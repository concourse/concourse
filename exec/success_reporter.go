package exec

type SuccessReporter interface {
	Subject(Step) Step

	Successful() bool
}
