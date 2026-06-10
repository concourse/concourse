package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/concourse/concourse/flag/binder"
)

var update = flag.Bool("update", false, "update the flag parity golden file")

func newRecords(t *testing.T) map[string][]binder.FlagInfo {
	t.Helper()

	registry := binder.NewRegistry(envPrefix)
	webCommand(registry)
	workerCommand(registry)
	migrateCommand(registry)
	quickstartCommand(registry)
	landWorkerCommand(registry)
	retireWorkerCommand(registry)
	generateKeyCommand(registry)

	records := map[string][]binder.FlagInfo{}
	for _, b := range registry.Binders() {
		infos := b.Flags()
		for i := range infos {
			normalize(&infos[i])
		}
		sortInfos(infos)
		records[b.Name()] = infos
	}

	return records
}

func sortInfos(infos []binder.FlagInfo) {
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
}

func normalize(info *binder.FlagInfo) {
	if len(info.Defaults) == 0 {
		info.Defaults = nil
	}
	if len(info.Choices) == 0 {
		info.Choices = nil
	}
}

func diffInfos(t *testing.T, command string, want, got []binder.FlagInfo) {
	t.Helper()

	wantByName := map[string]binder.FlagInfo{}
	for _, info := range want {
		wantByName[info.Name] = info
	}
	gotByName := map[string]binder.FlagInfo{}
	for _, info := range got {
		gotByName[info.Name] = info
	}

	for name, w := range wantByName {
		g, ok := gotByName[name]
		if !ok {
			t.Errorf("%s: flag --%s missing", command, name)
			continue
		}
		if fmt.Sprintf("%+v", w) != fmt.Sprintf("%+v", g) {
			t.Errorf("%s: flag --%s differs:\n  golden:  %+v\n  current: %+v", command, name, w, g)
		}
	}

	for name := range gotByName {
		if _, ok := wantByName[name]; !ok {
			t.Errorf("%s: flag --%s is new", command, name)
		}
	}
}

// TestFlagGolden guards the flag surface — names, shorthands,
// descriptions, env vars, defaults, requiredness, choices and visibility
// — against accidental drift, using a checked-in dump per platform. The
// golden file was generated from a parser built with the pre-migration
// go-flags + twentythousandtonnesofcrudeoil stack and verified identical
// to the cobra/binder command tree, so it also documents the historical
// behavior. Regenerate after deliberate flag changes with `go test -run
// TestFlagGolden -update ./cmd/concourse`.
func TestFlagGolden(t *testing.T) {
	golden := filepath.Join("testdata", fmt.Sprintf("flags_%s_%s.json", runtime.GOOS, runtime.GOARCH))

	current := newRecords(t)

	if *update {
		data, err := json.MarshalIndent(current, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(golden)
	if os.IsNotExist(err) {
		t.Skipf("no golden file for %s/%s; generate one with -update", runtime.GOOS, runtime.GOARCH)
	}
	if err != nil {
		t.Fatal(err)
	}

	var want map[string][]binder.FlagInfo
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}

	for command, wantInfos := range want {
		gotInfos, ok := current[command]
		if !ok {
			t.Errorf("command %s missing", command)
			continue
		}
		diffInfos(t, command, wantInfos, gotInfos)
	}

	for command := range current {
		if _, ok := want[command]; !ok {
			t.Errorf("command %s is not in the golden file; regenerate with -update", command)
		}
	}
}
