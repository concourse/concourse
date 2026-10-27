package healthserver

import (
	"fmt"
	"strconv"
	"strings"
)

type thresholdKind string

const (
	thresholdKindPlatform thresholdKind = "platform"
	thresholdKindTeam     thresholdKind = "team"
	thresholdKindTag      thresholdKind = "tag"
)

type ThresholdSeverity string

const (
	ThresholdSeverityCritical ThresholdSeverity = "critical"
	ThresholdSeverityWarn     ThresholdSeverity = "warn"
	ThresholdSeverityInfo     ThresholdSeverity = "info"
)

var validPlatforms = map[string]struct{}{
	"linux":   {},
	"windows": {},
	"darwin":  {},
}

type ThresholdEntry struct {
	Min      int
	Severity ThresholdSeverity
}

type MinWorkerThresholds struct {
	Platform map[string]ThresholdEntry
	Team     map[string]ThresholdEntry
	Tag      map[string]ThresholdEntry
}

func ParseMinWorkerThresholds(entries []string) (MinWorkerThresholds, error) {
	var t MinWorkerThresholds
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 4)
		if len(parts) != 4 {
			return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: expected format kind:name:count:severity", entry)
		}

		kind := thresholdKind(parts[0])
		name := parts[1]
		rawCount := parts[2]

		severity := ThresholdSeverity(parts[3])
		switch severity {
		case ThresholdSeverityCritical, ThresholdSeverityWarn, ThresholdSeverityInfo:
		default:
			return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: severity must be one of critical, warn, info", entry)
		}

		count, err := strconv.Atoi(rawCount)
		if err != nil || count < 0 {
			return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: count must be a non-negative integer", entry)
		}

		e := ThresholdEntry{Min: count, Severity: severity}

		switch kind {
		case thresholdKindPlatform:
			if name == "" {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: platform name must not be empty", entry)
			}

			if _, ok := validPlatforms[name]; !ok {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: platform must be one of linux, windows, darwin", entry)
			}

			if _, dup := t.Platform[name]; dup {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: duplicate platform %q", entry, name)
			}

			if t.Platform == nil {
				t.Platform = make(map[string]ThresholdEntry)
			}

			t.Platform[name] = e

		case thresholdKindTeam:
			if _, dup := t.Team[name]; dup {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: duplicate team %q", entry, name)
			}

			if t.Team == nil {
				t.Team = make(map[string]ThresholdEntry)
			}

			t.Team[name] = e

		case thresholdKindTag:
			if name == "" {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: tag name must not be empty", entry)
			}

			if _, dup := t.Tag[name]; dup {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: duplicate tag %q", entry, name)
			}

			if t.Tag == nil {
				t.Tag = make(map[string]ThresholdEntry)
			}

			t.Tag[name] = e

		default:
			return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-worker-scopes entry %q: unknown kind %q (must be platform, team, or tag)", entry, kind)
		}
	}
	return t, nil
}
