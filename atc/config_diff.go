package atc

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/aryann/difflib"
	"github.com/mgutz/ansi"
	"github.com/onsi/gomega/gexec"
	"sigs.k8s.io/yaml"
)

type Index interface {
	FindEquivalent(interface{}) (interface{}, bool)
	Slice() []interface{}
}

type Diffs []Diff

type Diff struct {
	Before interface{}
	After  interface{}
}

func name(v interface{}, nameOverride string) string {
	if nameOverride != "" {
		return nameOverride
	}
	return reflect.ValueOf(v).FieldByName("Name").String()
}

func (diff Diff) Render(to io.Writer, label, nameOverride string) {
	var payloadA, payloadB []byte = []byte{}, []byte{}
	var named, verb string
	if diff.Before != nil && diff.After != nil {
		verb, named = "changed", name(diff.Before, nameOverride)
		payloadA, _ = yaml.Marshal(diff.Before)
		payloadB, _ = yaml.Marshal(diff.After)
	} else if diff.Before != nil {
		verb, named = "been removed", name(diff.Before, nameOverride)
		payloadA, _ = yaml.Marshal(diff.Before)
	} else {
		verb, named = "been added", name(diff.After, nameOverride)
		payloadB, _ = yaml.Marshal(diff.After)
	}

	fmt.Fprintf(to, ansi.Color("%s %s has %s:", "yellow")+"\n", label, named, verb)
	renderDiff(to, string(payloadA), string(payloadB))
}

type GroupIndex GroupConfigs

func (index GroupIndex) Slice() []interface{} {
	slice := make([]interface{}, len(index))
	for i, object := range index {
		slice[i] = object
	}

	return slice
}

func (index GroupIndex) FindEquivalentWithOrder(obj interface{}) (interface{}, int, bool) {
	return GroupConfigs(index).Lookup(name(obj, ""))
}

type VarSourceIndex VarSourceConfigs

func (index VarSourceIndex) Slice() []interface{} {
	slice := make([]interface{}, len(index))
	for i, object := range index {
		slice[i] = object
	}

	return slice
}

func (index VarSourceIndex) FindEquivalent(obj interface{}) (interface{}, bool) {
	return VarSourceConfigs(index).Lookup(name(obj, ""))
}

type JobIndex JobConfigs

func (index JobIndex) Slice() []interface{} {
	slice := make([]interface{}, len(index))
	for i, object := range index {
		slice[i] = object
	}

	return slice
}

func (index JobIndex) FindEquivalent(obj interface{}) (interface{}, bool) {
	return JobConfigs(index).Lookup(name(obj, ""))
}

type ResourceIndex ResourceConfigs

func (index ResourceIndex) Slice() []interface{} {
	slice := make([]interface{}, len(index))
	for i, object := range index {
		slice[i] = object
	}

	return slice
}

func (index ResourceIndex) FindEquivalent(obj interface{}) (interface{}, bool) {
	return ResourceConfigs(index).Lookup(name(obj, ""))
}

type ResourceTypeIndex ResourceTypes

func (index ResourceTypeIndex) Slice() []interface{} {
	slice := make([]interface{}, len(index))
	for i, object := range index {
		slice[i] = object
	}

	return slice
}

func (index ResourceTypeIndex) FindEquivalent(obj interface{}) (interface{}, bool) {
	return ResourceTypes(index).Lookup(name(obj, ""))
}

func groupDiffIndices(oldIndex GroupIndex, newIndex GroupIndex) Diffs {
	diffs := Diffs{}

	for oldIndexNum, thing := range oldIndex.Slice() {
		newThing, newIndexNum, found := newIndex.FindEquivalentWithOrder(thing)
		if !found {
			diffs = append(diffs, Diff{
				Before: thing,
				After:  nil,
			})
			continue
		}

		if practicallyDifferent(thing, newThing) {
			diffs = append(diffs, Diff{
				Before: thing,
				After:  newThing,
			})
		}

		if oldIndexNum != newIndexNum {
			diffs = append(diffs, Diff{
				Before: thing,
				After:  newThing,
			})
		}
	}

	for _, thing := range newIndex.Slice() {
		_, _, found := oldIndex.FindEquivalentWithOrder(thing)
		if !found {
			diffs = append(diffs, Diff{
				Before: nil,
				After:  thing,
			})
			continue
		}
	}

	return diffs
}

func diffIndices(oldIndex Index, newIndex Index) Diffs {
	diffs := Diffs{}

	for _, thing := range oldIndex.Slice() {
		newThing, found := newIndex.FindEquivalent(thing)
		if !found {
			diffs = append(diffs, Diff{
				Before: thing,
				After:  nil,
			})
			continue
		}

		if practicallyDifferent(thing, newThing) {
			diffs = append(diffs, Diff{
				Before: thing,
				After:  newThing,
			})
		}
	}

	for _, thing := range newIndex.Slice() {
		_, found := oldIndex.FindEquivalent(thing)
		if !found {
			diffs = append(diffs, Diff{
				Before: nil,
				After:  thing,
			})
			continue
		}
	}

	return diffs
}

func diffDisplay(oldDisplay, newDisplay DisplayConfig) (Diff, bool) {
	nilDisplayConfig := DisplayConfig{}
	if oldDisplay == nilDisplayConfig {
		return Diff{
			Before: nil,
			After:  newDisplay,
		}, true
	} else if newDisplay == nilDisplayConfig {
		return Diff{
			Before: oldDisplay,
			After:  nil,
		}, true
	} else if practicallyDifferent(oldDisplay, newDisplay) {
		return Diff{
			Before: oldDisplay,
			After:  newDisplay,
		}, true
	}

	return Diff{}, false
}

func renderDiff(to io.Writer, a, b string) {
	diffs := difflib.Diff(strings.Split(a, "\n"), strings.Split(b, "\n"))
	indent := gexec.NewPrefixedWriter("\b\b", to)

	for _, diff := range diffs {
		text := diff.Payload

		switch diff.Delta {
		case difflib.RightOnly:
			fmt.Fprintf(indent, "%s %s\n", ansi.Color("+", "green"), ansi.Color(text, "green"))
		case difflib.LeftOnly:
			fmt.Fprintf(indent, "%s %s\n", ansi.Color("-", "red"), ansi.Color(text, "red"))
		case difflib.Common:
			fmt.Fprintf(to, "%s\n", text)
		}
	}
}

func practicallyDifferent(a, b interface{}) bool {
	if reflect.DeepEqual(a, b) {
		return false
	}

	// prevent silly things like 300 != 300.0 due to YAML vs. JSON
	// inconsistencies

	marshalledA, _ := yaml.Marshal(a)
	marshalledB, _ := yaml.Marshal(b)

	return !bytes.Equal(marshalledA, marshalledB)
}

func (c Config) Diff(out io.Writer, newConfig Config) bool {
	var diffExists bool

	indent := gexec.NewPrefixedWriter("  ", out)

	groupDiffs := groupDiffIndices(GroupIndex(c.Groups), GroupIndex(newConfig.Groups))
	if len(groupDiffs) > 0 {
		diffExists = true
		fmt.Fprintln(out, "groups:")

		for _, diff := range groupDiffs {
			diff.Render(indent, "group", "")
		}
	}

	varSourceDiffs := diffIndices(VarSourceIndex(c.VarSources), VarSourceIndex(newConfig.VarSources))
	if len(varSourceDiffs) > 0 {
		diffExists = true
		fmt.Println("variable source:")

		for _, diff := range varSourceDiffs {
			diff.Render(indent, "variable source", "")
		}
	}

	resourceDiffs := diffIndices(ResourceIndex(c.Resources), ResourceIndex(newConfig.Resources))
	if len(resourceDiffs) > 0 {
		diffExists = true
		fmt.Fprintln(out, "resources:")

		for _, diff := range resourceDiffs {
			diff.Render(indent, "resource", "")
		}
	}

	resourceTypeDiffs := diffIndices(ResourceTypeIndex(c.ResourceTypes), ResourceTypeIndex(newConfig.ResourceTypes))
	if len(resourceTypeDiffs) > 0 {
		diffExists = true
		fmt.Fprintln(out, "resource types:")

		for _, diff := range resourceTypeDiffs {
			diff.Render(indent, "resource type", "")
		}
	}

	jobDiffs := diffIndices(JobIndex(c.Jobs), JobIndex(newConfig.Jobs))
	if len(jobDiffs) > 0 {
		diffExists = true
		fmt.Fprintln(out, "jobs:")

		for _, diff := range jobDiffs {
			diff.Render(indent, "job", "")
		}
	}

	displayDiff, diff := diffDisplay(c.Display, newConfig.Display)
	if diff {
		diffExists = true
		displayDiff.Render(indent, "display", "configuration")
	}

	return diffExists
}
