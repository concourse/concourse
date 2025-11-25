package atc

var (
	EnableGlobalResources                bool
	EnableRedactSecrets                  bool
	EnableBuildRerunWhenWorkerDisappears bool
	EnableAcrossStep                     bool
	EnableCacheStreamedVolumes           bool
	EnableResourceCausality              bool
)

func FeatureFlags() map[string]bool {
	// If a feature flag is removed from this map, make sure it is also removed
	// from the corresponding type in Elm (web/elm/src/Concourse.elm -> FeatureFlags)
	return map[string]bool{
		"global_resources":       EnableGlobalResources,
		"redact_secrets":         EnableRedactSecrets,
		"build_rerun":            EnableBuildRerunWhenWorkerDisappears,
		"across_step":            EnableAcrossStep,
		"cache_streamed_volumes": EnableCacheStreamedVolumes,
		"resource_causality":     EnableResourceCausality,
	}
}
