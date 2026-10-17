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

type MinWorkerThresholds struct {
	Platform map[string]int
	Team     map[string]int
	Tag      map[string]int
}

func ParseMinWorkerThresholds(entries []string) (MinWorkerThresholds, error) {
	var t MinWorkerThresholds
	for _, raw := range entries {
		for _, entry := range strings.Split(raw, ";") {
			if entry == "" {
				continue
			}
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) != 3 {
			return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-workers entry %q: expected format kind:name:count", entry)
		}

		kind := thresholdKind(parts[0])
		name := parts[1]
		rawCount := parts[2]

		switch kind {
		case thresholdKindPlatform, thresholdKindTag:
			if name == "" {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-workers entry %q: %s name must not be empty", entry, kind)
			}
		case thresholdKindTeam:
		default:
			return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-workers entry %q: unknown kind %q (must be platform, team, or tag)", entry, kind)
		}

		count, err := strconv.Atoi(rawCount)
		if err != nil || count < 0 {
			return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-workers entry %q: count must be a non-negative integer", entry)
		}

		switch kind {
		case thresholdKindPlatform:
			if _, dup := t.Platform[name]; dup {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-workers entry %q: duplicate platform %q", entry, name)
			}
			if t.Platform == nil {
				t.Platform = make(map[string]int)
			}
			t.Platform[name] = count

		case thresholdKindTeam:
			if _, dup := t.Team[name]; dup {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-workers entry %q: duplicate team %q", entry, name)
			}
			if t.Team == nil {
				t.Team = make(map[string]int)
			}
			t.Team[name] = count

		case thresholdKindTag:
			if _, dup := t.Tag[name]; dup {
				return MinWorkerThresholds{}, fmt.Errorf("invalid --health-min-workers entry %q: duplicate tag %q", entry, name)
			}
			if t.Tag == nil {
				t.Tag = make(map[string]int)
			}
			t.Tag[name] = count
		}
		}
	}
	return t, nil
}
