package main

import (
	"github.com/concourse/concourse"
	"github.com/spf13/cobra"
)

var ConcourseCommand = &cobra.Command{
	Use:     "concourse",
	Short:   "c",
	Long:    `TODO`,
	Version: concourse.Version,
}

func init() {
	ConcourseCommand.AddCommand(WebCommand)
	ConcourseCommand.AddCommand(WorkerCommand)
	ConcourseCommand.AddCommand(MigrateCmd)
	ConcourseCommand.AddCommand(QuickstartCommand)
	ConcourseCommand.AddCommand(LandWorkerCommand)
	ConcourseCommand.AddCommand(RetireWorkerCommand)
	ConcourseCommand.AddCommand(GenerateKeyCommand)
}
