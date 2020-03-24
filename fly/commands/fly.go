package commands

import (
	"github.com/concourse/concourse/fly/rc"
)

type FlyCommand struct {
	Help HelpCommand `command:"help" description:"Print this help message"`

	Target       rc.TargetName       `short:"t" long:"target" description:"Concourse target name"`
	Targets      TargetsCommand      `command:"targets" alias:"ts" description:"List saved targets"`
	DeleteTarget DeleteTargetCommand `command:"delete-target" alias:"dtg" description:"Delete target"`
	EditTarget   EditTargetCommand   `command:"edit-target" alias:"etg" description:"Edit a target"`

	Version func() `short:"v" long:"version" description:"Print the version of Fly and exit"`

	Verbose bool `long:"verbose" description:"Print API requests and responses"`

	PrintTableHeaders bool `long:"print-table-headers" description:"Print table headers even for redirected output"`

	Login  LoginCommand  `command:"login" alias:"l" description:"Authenticate with the target"`
	Logout LogoutCommand `command:"logout" alias:"o" description:"Release authentication with the target"`
	Status StatusCommand `command:"status" description:"Login status"`
	Sync   SyncCommand   `command:"sync"  alias:"s" description:"Download and replace the current fly from the target"`

	ActiveUsers ActiveUsersCommand `command:"active-users" alias:"au" description:"List the active users since a date or for the past 2 months"`
	Userinfo    UserinfoCommand    `command:"userinfo" description:"User information"`

	Teams       TeamsCommand       `command:"teams" alias:"t" description:"List the configured teams"`
	GetTeam     GetTeamCommand     `command:"get-team"  alias:"gt" description:"Show team configuration"`
	SetTeam     SetTeamCommand     `command:"set-team"  alias:"st" description:"Create or modify a team to have the given credentials"`
	RenameTeam  RenameTeamCommand  `command:"rename-team"   alias:"rt" description:"Rename a team"`
	DestroyTeam DestroyTeamCommand `command:"destroy-team"  alias:"dt" description:"Destroy a team and delete all of its data"`

	Checklist ChecklistCommand `command:"checklist" alias:"cl" description:"Print a Checkfile of the given pipeline"`

	Execute ExecuteCommand `command:"execute" alias:"e" description:"Execute a one-off build using local bits"`
	Watch   WatchCommand   `command:"watch"   alias:"w" description:"Stream a build's output"`

	Containers ContainersCommand `command:"containers" alias:"cs" description:"Print the active containers"`
	Hijack     HijackCommand     `command:"hijack"     alias:"intercept" alias:"i" description:"Execute a command in a container"`

	Jobs        JobsCommand        `command:"jobs"      alias:"js" description:"List the jobs in the pipelines"`
	PauseJob    PauseJobCommand    `command:"pause-job" alias:"pj" description:"Pause a job"`
	UnpauseJob  UnpauseJobCommand  `command:"unpause-job" alias:"uj" description:"Unpause a job"`
	ScheduleJob ScheduleJobCommand `command:"schedule-job" alias:"sj" description:"Request the scheduler to run for a job. Introduced as a recovery command for the v6.0 scheduler."`

	Pipelines        PipelinesCommand        `command:"pipelines"           alias:"ps"   description:"List the configured pipelines"`
	DestroyPipeline  DestroyPipelineCommand  `command:"destroy-pipeline"    alias:"dp"   description:"Destroy a pipeline"`
	GetPipeline      GetPipelineCommand      `command:"get-pipeline"        alias:"gp"   description:"Get a pipeline's current configuration"`
	SetPipeline      SetPipelineCommand      `command:"set-pipeline"        alias:"sp"   description:"Create or update a pipeline's configuration"`
	PausePipeline    PausePipelineCommand    `command:"pause-pipeline"      alias:"pp"   description:"Pause a pipeline"`
	ArchivePipeline  ArchivePipelineCommand  `command:"archive-pipeline"    alias:"ap"   description:"Archive a pipeline"`
	UnpausePipeline  UnpausePipelineCommand  `command:"unpause-pipeline"    alias:"up"   description:"Un-pause a pipeline"`
	ExposePipeline   ExposePipelineCommand   `command:"expose-pipeline"     alias:"ep"   description:"Make a pipeline publicly viewable"`
	HidePipeline     HidePipelineCommand     `command:"hide-pipeline"       alias:"hp"   description:"Hide a pipeline from the public"`
	RenamePipeline   RenamePipelineCommand   `command:"rename-pipeline"     alias:"rp"   description:"Rename a pipeline"`
	ValidatePipeline ValidatePipelineCommand `command:"validate-pipeline"   alias:"vp"   description:"Validate a pipeline config"`
	FormatPipeline   FormatPipelineCommand   `command:"format-pipeline"     alias:"fp"   description:"Format a pipeline config"`
	OrderPipelines   OrderPipelinesCommand   `command:"order-pipelines"     alias:"op"   description:"Orders pipelines"`

	Resources              ResourcesCommand              `command:"resources"                  alias:"rs"   description:"List the resources in the pipeline"`
	ResourceVersions       ResourceVersionsCommand       `command:"resource-versions"          alias:"rvs"  description:"List the versions of a resource"`
	CheckResource          CheckResourceCommand          `command:"check-resource"             alias:"cr"   description:"Check a resource"`
	PinResource            PinResourceCommand            `command:"pin-resource"               alias:"pr"   description:"Pin a version to a resource"`
	UnpinResource          UnpinResourceCommand          `command:"unpin-resource"             alias:"ur"   description:"Unpin a resource"`
	EnableResourceVersion  EnableResourceVersionCommand  `command:"enable-resource-version"    alias:"erv"  description:"Enable a version of a resource"`
	DisableResourceVersion DisableResourceVersionCommand `command:"disable-resource-version"   alias:"drv"  description:"Disable a version of a resource"`

	CheckResourceType CheckResourceTypeCommand `command:"check-resource-type" alias:"crt"  description:"Check a resource-type"`

	ClearTaskCache ClearTaskCacheCommand `command:"clear-task-cache" alias:"ctc" description:"Clears cache from a task container"`

	Builds     BuildsCommand     `command:"builds"      alias:"bs" description:"List builds data"`
	AbortBuild AbortBuildCommand `command:"abort-build" alias:"ab" description:"Abort a build"`
	RerunBuild RerunBuildCommand `command:"rerun-build" alias:"rb" description:"Rerun a build"`

	TriggerJob TriggerJobCommand `command:"trigger-job" alias:"tj" description:"Start a job in a pipeline"`

	Volumes VolumesCommand `command:"volumes" alias:"vs" description:"List the active volumes"`

	Workers     WorkersCommand     `command:"workers" alias:"ws" description:"List the registered workers"`
	LandWorker  LandWorkerCommand  `command:"land-worker" alias:"lw" description:"Land a worker"`
	PruneWorker PruneWorkerCommand `command:"prune-worker" alias:"pw" description:"Prune a stalled, landing, landed, or retiring worker"`

	Curl CurlCommand `command:"curl" alias:"c" description:"curl the api"`

	Completion CompletionCommand `command:"completion" description:"generate shell completion code"`
}

var Fly FlyCommand
