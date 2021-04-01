//go:generate go run github.com/markbates/pkger/cmd/pkger -o ./cmd/concourse

package main

func main() {
	err := ConcourseCommand.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
