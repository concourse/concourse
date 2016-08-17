package commands

import "github.com/concourse/fly/rc"

type FlyCommand struct {
	Help HelpCommand `command:"help" description:"Print this help message"`

	Target  rc.TargetName  `short:"t" long:"target" description:"Concourse target name"`
	Targets TargetsCommand `command:"targets" alias:"ts" description:"List saved targets"`

	Version func() `short:"v" long:"version" description:"Print the version of Fly and exit"`

	Login LoginCommand `command:"login" alias:"l" description:"Authenticate with the target"`
	Sync  SyncCommand  `command:"sync"  alias:"s" description:"Download and replace the current fly from the target"`

	SetTeam SetTeamCommand `command:"set-team"  alias:"st" description:"Create or modify a team to have the given credentials"`

	Checklist ChecklistCommand `command:"checklist" alias:"cl" description:"Print a Checkfile of the given pipeline"`

	Execute ExecuteCommand `command:"execute" alias:"e" description:"Execute a one-off build using local bits"`
	Watch   WatchCommand   `command:"watch"   alias:"w" description:"Stream a build's output"`

	Containers ContainersCommand `command:"containers" alias:"cs" description:"Print the active containers"`
	Hijack     HijackCommand     `command:"hijack"     alias:"intercept" alias:"i" description:"Execute a command in a container"`

	PauseJob   PauseJobCommand   `command:"pause-job" alias:"pj" description:"Pause a job"`
	UnpauseJob UnpauseJobCommand `command:"unpause-job" alias:"uj" description:"Unpause a job"`

	Pipelines       PipelinesCommand       `command:"pipelines"        alias:"ps" description:"List the configured pipelines"`
	DestroyPipeline DestroyPipelineCommand `command:"destroy-pipeline" alias:"dp" description:"Destroy a pipeline"`
	GetPipeline     GetPipelineCommand     `command:"get-pipeline"     alias:"gp" description:"Get a pipeline's current configuration"`
	SetPipeline     SetPipelineCommand     `command:"set-pipeline"     alias:"sp" description:"Create or update a pipeline's configuration"`
	PausePipeline   PausePipelineCommand   `command:"pause-pipeline"   alias:"pp" description:"Pause a pipeline"`
	UnpausePipeline UnpausePipelineCommand `command:"unpause-pipeline" alias:"up" description:"Un-pause a pipeline"`
	RevealPipeline  RevealPipelineCommand  `command:"reveal-pipeline"  alias:"rp" description:"Reveal a pipeline"`
	ConcealPipeline ConcealPipelineCommand `command:"conceal-pipeline" alias:"cp" description:"Conceal a pipeline"`
	RenamePipeline  RenamePipelineCommand  `command:"rename-pipeline"  alias:"rp" description:"Rename a pipeline"`

	CheckResource   CheckResourceCommand   `command:"check-resource"  alias:"cr" description:"Check a resource"`
	PauseResource   PauseResourceCommand   `command:"pause-resource"  alias:"pr" description:"Pause a resource"`
	UnpauseResource UnpauseResourceCommand `command:"unpause-resource"  alias:"ur" description:"Unpause a resource"`

	Builds     BuildsCommand     `command:"builds"      alias:"bs" description:"List builds data"`
	AbortBuild AbortBuildCommand `command:"abort-build" alias:"ab" description:"Abort a build"`

	TriggerJob TriggerJobCommand `command:"trigger-job" alias:"tj" description:"Start a job in a pipeline"`

	Volumes VolumesCommand `command:"volumes" alias:"vs" description:"List the active volumes"`
	Workers WorkersCommand `command:"workers" alias:"ws" description:"List the registered workers"`
}

var Fly FlyCommand
