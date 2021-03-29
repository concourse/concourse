//go:generate go run github.com/markbates/pkger/cmd/pkger -o ./cmd/concourse

package main

func main() {
	ConcourseCommand.Execute()
}
