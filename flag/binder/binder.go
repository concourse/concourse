// Package binder registers pflag flags and CONCOURSE_* environment
// variables from the go-flags-tagged config structs used by the concourse
// commands, preserving the parsing behavior of go-flags +
// twentythousandtonnesofcrudeoil: flag and env var names, defaults,
// choice validation, required-flag semantics (satisfiable by env or
// default), comma-delimited collections from env, and custom
// UnmarshalFlag types.
package binder

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Registry tracks every environment variable bound by any Binder created
// from it. Before a command executes, all registered variables matching
// the prefix are cleared from the environment — across all commands, not
// just the one running — mirroring twentythousandtonnesofcrudeoil. The
// worker relies on this: leftover CONCOURSE_GARDEN_* vars are forwarded
// verbatim to the gdn binary, so consumed ones must disappear.
type Registry struct {
	envPrefix string
	envKeys   map[string]struct{}
	binders   []*Binder
}

// NewRegistry creates a registry deriving env vars with the given prefix
// (e.g. "CONCOURSE_"). An empty prefix disables env var derivation;
// explicit `env:"..."` tags are still honored, as with plain go-flags.
func NewRegistry(envPrefix string) *Registry {
	return &Registry{
		envPrefix: envPrefix,
		envKeys:   map[string]struct{}{},
	}
}

// Binder accumulates the flags of one command.
func (r *Registry) Binder(fs *pflag.FlagSet) *Binder {
	v := viper.New()
	// go-flags treats an env var set to the empty string as set
	v.AllowEmptyEnv(true)

	b := &Binder{
		registry: r,
		fs:       fs,
		viper:    v,
		flags:    map[string]*boundFlag{},
	}
	r.binders = append(r.binders, b)

	return b
}

// Binders returns every binder created from this registry, one per
// command.
func (r *Registry) Binders() []*Binder {
	return r.binders
}

// ClearEnv unsets every registered env var that matches the registry
// prefix. Explicitly-tagged keys like http_proxy don't match and survive.
func (r *Registry) ClearEnv() {
	if r.envPrefix == "" {
		return
	}
	for key := range r.envKeys {
		if strings.HasPrefix(key, r.envPrefix) {
			os.Unsetenv(key)
		}
	}
}

type Binder struct {
	registry *Registry
	fs       *pflag.FlagSet
	viper    *viper.Viper
	flags    map[string]*boundFlag
	ordered  []*boundFlag
	sections []*usageSection
}

// usageSection groups flags under a help heading, mirroring go-flags'
// `group:` tag sections. The unnamed section holds the command's own
// flags and renders first, without a heading.
type usageSection struct {
	title string
	flags []*boundFlag
}

type boundFlag struct {
	name         string
	envKey       string
	required     bool
	defaults     []string
	isCollection bool
	value        *flagValue
	section      *usageSection
	envApplied   bool
}

// Bind walks obj (a pointer to a go-flags-tagged struct), allocating nil
// struct-pointer groups and registering a flag for every `long`-tagged
// field, prefixed with prefix (e.g. "" or "worker-"). Nested `namespace`
// tags extend the prefix with a "-" delimiter, as go-flags did with
// NamespaceDelimiter = "-".
func (b *Binder) Bind(obj any, prefix string) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("binder: Bind expects a non-nil pointer to struct, got %T", obj)
	}

	return b.bindStruct(v.Elem(), prefix, b.defaultSection())
}

// MustBind is Bind for command constructors, where a failure is a
// programming error in the struct tags.
func (b *Binder) MustBind(obj any, prefix string) {
	if err := b.Bind(obj, prefix); err != nil {
		panic(err)
	}
}

// BindGroup binds obj like Bind, but places its flags under the given
// help heading — the equivalent of go-flags' Group.AddGroup, used for
// dynamically-registered groups (credential managers, metric emitters,
// policy check agents, auth connectors). Nested `group:` tags inside obj
// still start their own sections.
func (b *Binder) BindGroup(title string, obj any, prefix string) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("binder: BindGroup expects a non-nil pointer to struct, got %T", obj)
	}

	return b.bindStruct(v.Elem(), prefix, b.section(title))
}

func (b *Binder) MustBindGroup(title string, obj any, prefix string) {
	if err := b.BindGroup(title, obj, prefix); err != nil {
		panic(err)
	}
}

// defaultSection returns the unnamed section holding the command's own
// flags, creating it at the front so it always renders first.
func (b *Binder) defaultSection() *usageSection {
	if len(b.sections) > 0 && b.sections[0].title == "" {
		return b.sections[0]
	}

	s := &usageSection{}
	b.sections = append([]*usageSection{s}, b.sections...)
	return s
}

func (b *Binder) section(title string) *usageSection {
	s := &usageSection{title: title}
	b.sections = append(b.sections, s)
	return s
}

func (b *Binder) bindStruct(s reflect.Value, prefix string, sec *usageSection) error {
	stype := s.Type()

	for i := 0; i < stype.NumField(); i++ {
		field := stype.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		tags := parseMultiTag(string(field.Tag))
		if truthy(tags.Get("no-flag")) {
			continue
		}

		long := tags.Get("long")
		short := tags.Get("short")
		fv := s.Field(i)

		if long == "" && short == "" {
			// not an option; descend into structs and pointers to structs
			target := fv
			ftype := field.Type

			if ftype.Kind() == reflect.Pointer && ftype.Elem().Kind() == reflect.Struct {
				if fv.IsNil() {
					fv.Set(reflect.New(ftype.Elem()))
				}
				target = fv.Elem()
				ftype = ftype.Elem()
			}

			if ftype.Kind() == reflect.Struct {
				sub := prefix
				if ns := tags.Get("namespace"); ns != "" {
					sub = prefix + ns + "-"
				}
				subSec := sec
				if title := tags.Get("group"); title != "" {
					subSec = b.section(title)
				}
				if err := b.bindStruct(target, sub, subSec); err != nil {
					return err
				}
			}

			continue
		}

		if field.Type.Kind() == reflect.Func {
			// e.g. the old root-level version callback; commands handle
			// these natively
			continue
		}

		if long == "" {
			return fmt.Errorf("binder: field %s.%s has only a short flag name; a long name is required", stype, field.Name)
		}

		if err := b.bindField(fv, field, tags, prefix, sec); err != nil {
			return err
		}
	}

	return nil
}

func (b *Binder) bindField(fv reflect.Value, field reflect.StructField, tags multiTag, prefix string, sec *usageSection) error {
	name := prefix + tags.Get("long")
	short := tags.Get("short")
	defaults := tags.GetMany("default")
	boolish := isBoolType(field.Type)

	if _, exists := b.flags[name]; exists {
		return fmt.Errorf("binder: flag --%s registered twice", name)
	}

	if boolish && len(defaults) > 0 {
		return fmt.Errorf("binder: boolean flag --%s may not have default values, they always default to `false' and can only be turned on", name)
	}

	rawKind := field.Type.Kind()

	value := &flagValue{
		dest:         fv,
		choices:      tags.GetMany("choice"),
		optName:      optString(short, name),
		valueName:    tags.Get("value-name"),
		boolish:      boolish,
		isCollection: rawKind == reflect.Map || rawKind == reflect.Slice,
	}

	for _, d := range defaults {
		if err := value.Set(d); err != nil {
			return fmt.Errorf("binder: invalid default value for --%s: %s", name, err)
		}
	}
	value.changed = false
	value.defValue = strings.Join(defaults, ", ")

	if rawKind == reflect.Map && fv.IsNil() {
		// go-flags leaves maps allocated after parsing even when no value
		// was provided; code like web's populateSharedFlags assigns into
		// them
		fv.Set(reflect.MakeMap(field.Type))
	}

	f := b.fs.VarPF(value, name, short, tags.Get("description"))
	if boolish {
		f.NoOptDefVal = "true"
	}
	if truthy(tags.Get("hidden")) {
		f.Hidden = true
	}

	envKey := tags.Get("env")
	if envKey == "" && b.registry.envPrefix != "" {
		envKey = b.registry.envPrefix + envName(name)
	}
	if envKey != "" {
		if err := b.viper.BindEnv(name, envKey); err != nil {
			return err
		}
		b.registry.envKeys[envKey] = struct{}{}
	}

	bf := &boundFlag{
		name:         name,
		envKey:       envKey,
		required:     truthy(tags.Get("required")),
		defaults:     defaults,
		isCollection: value.isCollection,
		value:        value,
		section:      sec,
	}
	b.flags[name] = bf
	b.ordered = append(b.ordered, bf)
	sec.flags = append(sec.flags, bf)

	return nil
}

// Lessen drops the required marker from the named flags. It panics on
// unknown names, like the FindOptionByLongName(...).Required = false it
// replaces would have.
func (b *Binder) Lessen(names ...string) {
	for _, name := range names {
		bf, ok := b.flags[name]
		if !ok {
			panic(fmt.Sprintf("binder: cannot lessen requirement of unknown flag --%s", name))
		}
		bf.required = false
	}
}

// Finish completes parsing for a command after the command line has been
// parsed: values from the environment are applied to all flags not set on
// the command line, required flags are checked (an env var or a tag
// default satisfies a requirement, as in go-flags), and all registered
// prefix-matching env vars are cleared from the environment.
func (b *Binder) Finish() error {
	if err := b.applyEnv(); err != nil {
		return err
	}

	if err := b.checkRequired(); err != nil {
		return err
	}

	b.registry.ClearEnv()

	return nil
}

func (b *Binder) applyEnv() error {
	for _, bf := range b.ordered {
		if bf.envKey == "" || b.fs.Changed(bf.name) {
			continue
		}

		if !b.viper.IsSet(bf.name) {
			continue
		}

		raw := b.viper.GetString(bf.name)

		values := []string{raw}
		if bf.isCollection {
			values = strings.Split(raw, ",")
		}

		for _, value := range values {
			if err := bf.value.Set(value); err != nil {
				return fmt.Errorf("invalid value for %s: %s", bf.envKey, err)
			}
		}

		bf.envApplied = true
	}

	return nil
}

func (b *Binder) checkRequired() error {
	var names []string
	for _, bf := range b.ordered {
		if bf.required && !b.fs.Changed(bf.name) && !bf.envApplied && len(bf.defaults) == 0 {
			names = append(names, "`"+bf.value.optName+"'")
		}
	}

	if len(names) == 0 {
		return nil
	}

	sort.Strings(names)

	if len(names) == 1 {
		return fmt.Errorf("the required flag %s was not specified", names[0])
	}

	return fmt.Errorf("the required flags %s and %s were not specified",
		strings.Join(names[:len(names)-1], ", "), names[len(names)-1])
}

// FlagInfo describes a bound flag for introspection, used by the parity
// tests guarding against drift from the historical go-flags definitions.
type FlagInfo struct {
	Name        string   `json:"name"`
	Short       string   `json:"short,omitempty"`
	Description string   `json:"description,omitempty"`
	EnvKey      string   `json:"env,omitempty"`
	Defaults    []string `json:"defaults,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Choices     []string `json:"choices,omitempty"`
	Hidden      bool     `json:"hidden,omitempty"`
	ValueName   string   `json:"valueName,omitempty"`
	Group       string   `json:"group,omitempty"`
}

// Name returns the name of the flag set this binder registers into,
// which for cobra commands is the command name.
func (b *Binder) Name() string {
	return b.fs.Name()
}

// Flags returns a description of every bound flag, in registration order.
func (b *Binder) Flags() []FlagInfo {
	infos := make([]FlagInfo, 0, len(b.ordered))
	for _, bf := range b.ordered {
		f := b.fs.Lookup(bf.name)
		infos = append(infos, FlagInfo{
			Name:        bf.name,
			Short:       f.Shorthand,
			Description: f.Usage,
			EnvKey:      bf.envKey,
			Defaults:    bf.defaults,
			Required:    bf.required,
			Choices:     bf.value.choices,
			Hidden:      f.Hidden,
			ValueName:   bf.value.valueName,
			Group:       bf.section.title,
		})
	}
	return infos
}
