package atc

import "time"

const (
	ComponentScheduler                  = "scheduler"
	ComponentBuildTracker               = "tracker"
	ComponentLidarScanner               = "scanner"
	ComponentBuildReaper                = "reaper"
	ComponentSyslogDrainer              = "drainer"
	ComponentCollectorAccessTokens      = "collector_access_tokens"
	ComponentCollectorArtifacts         = "collector_artifacts"
	ComponentCollectorBuilds            = "collector_builds"
	ComponentCollectorCheckSessions     = "collector_check_sessions"
	ComponentCollectorChecks            = "collector_checks"
	ComponentCollectorContainers        = "collector_containers"
	ComponentCollectorResourceCacheUses = "collector_resource_cache_uses"
	ComponentCollectorResourceCaches    = "collector_resource_caches"
	ComponentCollectorTaskCaches        = "collector_task_caches"
	ComponentCollectorResourceConfigs   = "collector_resource_configs"
	ComponentCollectorVolumes           = "collector_volumes"
	ComponentCollectorWorkers           = "collector_workers"
	ComponentCollectorPipelines         = "collector_pipelines"
	ComponentPipelinePauser             = "pipeline_pauser"
)

type Component struct {
	Name     string
	Interval time.Duration
}
