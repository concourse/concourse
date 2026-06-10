package binder_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/concourse/concourse/flag/binder"
	"github.com/spf13/pflag"
)

type testURL struct {
	url string
}

func (u *testURL) UnmarshalFlag(value string) error {
	if !strings.Contains(value, "://") {
		return fmt.Errorf("missing scheme in '%s'", value)
	}
	u.url = value
	return nil
}

type testRoute string

func (r *testRoute) UnmarshalFlag(value string) error {
	if value != "ListAllJobs" && value != "CreateBuild" {
		return fmt.Errorf("'%s' is not a valid action", value)
	}
	*r = testRoute(value)
	return nil
}

type nestedConfig struct {
	Host string `long:"host" default:"127.0.0.1" description:"The host."`
	Port uint16 `long:"port" default:"5432"`
}

type groupedCommand struct {
	Name string `long:"name" description:"A name."`

	Nested nestedConfig `group:"Nested Configuration" namespace:"nested"`

	Inline struct {
		Value string `long:"inline-value"`
	} `group:"Inline group without namespace"`

	Pointer *nestedConfig `group:"Pointer group" namespace:"ptr"`
}

type kitchenSink struct {
	Str      string        `long:"str" default:"hello"`
	Num      int           `long:"num" default:"42"`
	Unsigned uint16        `long:"unsigned"`
	Float    float64       `long:"float" default:"0.5"`
	Dur      time.Duration `long:"dur" default:"5m"`
	Bool     bool          `long:"bool"`

	URL    *testURL `long:"url"`
	ValURL testURL  `long:"val-url"`

	Slice        []string          `long:"slice"`
	SliceDef     []string          `long:"slice-def" default:"a" default:"b"`
	Map          map[string]string `long:"map"`
	RouteMap     map[testRoute]int `long:"route-map"`
	URLSlice     []testURL         `long:"url-slice"`
	Choice       string            `long:"choice" default:"gzip" choice:"gzip" choice:"zstd" choice:"raw"`
	SliceChoice  []string          `long:"slice-choice" choice:"x" choice:"y"`
	Hidden       bool              `long:"sneaky" hidden:"true"`
	Short        string            `short:"t" long:"type"`
	PtrInt       *int              `long:"ptr-int"`
	ProxyEnvOnly string            `long:"proxy" env:"test_proxy"`
}

func newBinder(t *testing.T, prefix string) (*binder.Registry, *binder.Binder, *pflag.FlagSet) {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	reg := binder.NewRegistry(prefix)
	return reg, reg.Binder(fs), fs
}

func parse(t *testing.T, fs *pflag.FlagSet, b *binder.Binder, args ...string) error {
	t.Helper()
	if err := fs.Parse(args); err != nil {
		return err
	}
	return b.Finish()
}

func TestScalarParsing(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b,
		"--str", "world",
		"--num", "7",
		"--unsigned", "8080",
		"--float", "1.25",
		"--dur", "90s",
		"--bool",
		"--url", "https://example.com",
		"--val-url", "https://example.org",
		"--ptr-int", "3",
	)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Str != "world" || cfg.Num != 7 || cfg.Unsigned != 8080 || cfg.Float != 1.25 {
		t.Errorf("scalars not parsed: %+v", cfg)
	}
	if cfg.Dur != 90*time.Second {
		t.Errorf("duration = %s", cfg.Dur)
	}
	if !cfg.Bool {
		t.Error("bool flag not set")
	}
	if cfg.URL == nil || cfg.URL.url != "https://example.com" {
		t.Errorf("pointer unmarshaler = %+v", cfg.URL)
	}
	if cfg.ValURL.url != "https://example.org" {
		t.Errorf("value unmarshaler = %+v", cfg.ValURL)
	}
	if cfg.PtrInt == nil || *cfg.PtrInt != 3 {
		t.Errorf("ptr-int = %v", cfg.PtrInt)
	}
}

func TestDefaults(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}
	if err := parse(t, fs, b); err != nil {
		t.Fatal(err)
	}

	if cfg.Str != "hello" || cfg.Num != 42 || cfg.Float != 0.5 || cfg.Dur != 5*time.Minute {
		t.Errorf("defaults not applied: %+v", cfg)
	}
	if got := fmt.Sprintf("%v", cfg.SliceDef); got != "[a b]" {
		t.Errorf("multiple default tags = %v", cfg.SliceDef)
	}
	if cfg.Choice != "gzip" {
		t.Errorf("choice default = %q", cfg.Choice)
	}
	if cfg.Map == nil {
		t.Error("map should be allocated after parsing, like go-flags")
	}
	if cfg.URL != nil {
		t.Error("unset pointer option should stay nil")
	}
}

func TestSliceReplaceThenAppend(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	// first CLI occurrence clears the tag default, further ones append
	if err := parse(t, fs, b, "--slice-def", "x", "--slice-def", "y"); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%v", cfg.SliceDef); got != "[x y]" {
		t.Errorf("slice = %v, want [x y]", cfg.SliceDef)
	}
}

func TestMapParsing(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b, "--map", "a:1", "--map", "b:2", "--route-map", "ListAllJobs:5")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Map["a"] != "1" || cfg.Map["b"] != "2" {
		t.Errorf("map = %v", cfg.Map)
	}
	if cfg.RouteMap[testRoute("ListAllJobs")] != 5 {
		t.Errorf("route map = %v", cfg.RouteMap)
	}
}

func TestMapCustomKeyValidation(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b, "--route-map", "InvalidAction:0")
	if err == nil || !strings.Contains(err.Error(), "'InvalidAction' is not a valid action") {
		t.Errorf("err = %v", err)
	}
}

func TestChoiceValidation(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b, "--choice", "bogus")
	want := "Invalid value `bogus' for option `--choice'. Allowed values are: gzip, zstd or raw"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestNamespacesAndGroups(t *testing.T) {
	cfg := &groupedCommand{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, "outer-"); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b,
		"--outer-name", "n",
		"--outer-nested-host", "h",
		"--outer-inline-value", "i",
		"--outer-ptr-port", "9",
	)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Name != "n" || cfg.Nested.Host != "h" || cfg.Inline.Value != "i" {
		t.Errorf("namespaced flags not bound: %+v", cfg)
	}
	if cfg.Pointer == nil || cfg.Pointer.Port != 9 {
		t.Errorf("pointer group = %+v", cfg.Pointer)
	}
	if cfg.Pointer.Host != "127.0.0.1" {
		t.Errorf("pointer group default = %q", cfg.Pointer.Host)
	}
}

func TestEnvPrecedence(t *testing.T) {
	t.Setenv("TESTPREFIX_STR", "from-env")
	t.Setenv("TESTPREFIX_NUM", "1000")

	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	// CLI wins over env; env wins over default
	if err := parse(t, fs, b, "--num", "7"); err != nil {
		t.Fatal(err)
	}
	if cfg.Str != "from-env" {
		t.Errorf("env should override default: %q", cfg.Str)
	}
	if cfg.Num != 7 {
		t.Errorf("CLI should override env: %d", cfg.Num)
	}
}

func TestEmptyEnvCountsAsSet(t *testing.T) {
	t.Setenv("TESTPREFIX_STR", "")
	t.Setenv("TESTPREFIX_BOOL", "")

	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}
	if err := parse(t, fs, b); err != nil {
		t.Fatal(err)
	}

	if cfg.Str != "" {
		t.Errorf("empty env should clear the string default, got %q", cfg.Str)
	}
	if !cfg.Bool {
		t.Error("empty env should turn a bool flag on, as in go-flags")
	}
}

func TestEnvCollections(t *testing.T) {
	t.Setenv("TESTPREFIX_SLICE_DEF", "x,y,z")
	t.Setenv("TESTPREFIX_MAP", "a:1,b:2")

	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}
	if err := parse(t, fs, b); err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprintf("%v", cfg.SliceDef); got != "[x y z]" {
		t.Errorf("env slice should replace defaults: %v", cfg.SliceDef)
	}
	if cfg.Map["a"] != "1" || cfg.Map["b"] != "2" {
		t.Errorf("env map = %v", cfg.Map)
	}
}

func TestEnvInvalidValue(t *testing.T) {
	t.Setenv("TESTPREFIX_ROUTE_MAP", "InvalidAction:0")

	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b)
	if err == nil ||
		!strings.Contains(err.Error(), "TESTPREFIX_ROUTE_MAP") ||
		!strings.Contains(err.Error(), "'InvalidAction' is not a valid action") {
		t.Errorf("err = %v", err)
	}
}

func TestEnvChoiceValidation(t *testing.T) {
	t.Setenv("TESTPREFIX_CHOICE", "bogus")

	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b)
	if err == nil || !strings.Contains(err.Error(), "Allowed values are: gzip, zstd or raw") {
		t.Errorf("env values must pass choice validation like go-flags: %v", err)
	}
}

func TestExplicitEnvTag(t *testing.T) {
	t.Setenv("test_proxy", "http://proxy")

	cfg := &kitchenSink{}
	reg, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}
	if err := parse(t, fs, b); err != nil {
		t.Fatal(err)
	}

	if cfg.ProxyEnvOnly != "http://proxy" {
		t.Errorf("explicit env tag not honored: %q", cfg.ProxyEnvOnly)
	}

	// non-prefixed keys survive ClearEnv
	reg.ClearEnv()
	if _, ok := os.LookupEnv("test_proxy"); !ok {
		t.Error("ClearEnv should only unset prefix-matching keys")
	}
}

func TestClearEnvAcrossBinders(t *testing.T) {
	t.Setenv("TESTPREFIX_STR", "x")
	t.Setenv("TESTPREFIX_NESTED_HOST", "y")
	t.Setenv("TESTPREFIX_UNREGISTERED", "z")

	reg := binder.NewRegistry("TESTPREFIX_")

	fs1 := pflag.NewFlagSet("one", pflag.ContinueOnError)
	b1 := reg.Binder(fs1)
	if err := b1.Bind(&kitchenSink{}, ""); err != nil {
		t.Fatal(err)
	}

	fs2 := pflag.NewFlagSet("two", pflag.ContinueOnError)
	b2 := reg.Binder(fs2)
	if err := b2.Bind(&groupedCommand{}, ""); err != nil {
		t.Fatal(err)
	}

	// running command one clears command two's env vars as well
	if err := fs1.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if err := b1.Finish(); err != nil {
		t.Fatal(err)
	}

	if _, ok := os.LookupEnv("TESTPREFIX_STR"); ok {
		t.Error("own env var should be cleared")
	}
	if _, ok := os.LookupEnv("TESTPREFIX_NESTED_HOST"); ok {
		t.Error("other commands' env vars should be cleared too")
	}
	if _, ok := os.LookupEnv("TESTPREFIX_UNREGISTERED"); !ok {
		t.Error("unregistered env vars must survive for the worker to forward them to gdn")
	}
}

type requiredCommand struct {
	HostKey  string `long:"host-key" required:"true"`
	FilePath string `short:"f" long:"filename" required:"true"`
	WithDef  string `long:"with-def" required:"true" default:"d"`
	Plain    string `long:"plain"`
}

func TestRequired(t *testing.T) {
	cfg := &requiredCommand{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b)
	want := "the required flags `--host-key' and `-f, --filename' were not specified"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestRequiredSingle(t *testing.T) {
	cfg := &requiredCommand{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	err := parse(t, fs, b, "--filename", "f")
	want := "the required flag `--host-key' was not specified"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestRequiredSatisfiedByEnv(t *testing.T) {
	t.Setenv("TESTPREFIX_HOST_KEY", "key")

	cfg := &requiredCommand{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	if err := parse(t, fs, b, "--filename", "f"); err != nil {
		t.Fatalf("env var should satisfy a required flag: %v", err)
	}
	if cfg.HostKey != "key" {
		t.Errorf("host key = %q", cfg.HostKey)
	}
}

func TestLessen(t *testing.T) {
	cfg := &requiredCommand{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	b.Lessen("host-key", "filename")

	if err := parse(t, fs, b); err != nil {
		t.Fatal(err)
	}
}

func TestLessenUnknownPanics(t *testing.T) {
	cfg := &requiredCommand{}
	_, b, _ := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if recover() == nil {
			t.Error("Lessen of unknown flag should panic")
		}
	}()
	b.Lessen("nope")
}

func TestShortAndHidden(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	if err := parse(t, fs, b, "-t", "rsa"); err != nil {
		t.Fatal(err)
	}
	if cfg.Short != "rsa" {
		t.Errorf("short flag = %q", cfg.Short)
	}

	f := fs.Lookup("sneaky")
	if f == nil || !f.Hidden {
		t.Error("hidden tag should mark the pflag hidden")
	}
}

func TestNoEnvWithoutPrefix(t *testing.T) {
	t.Setenv("STR", "x")
	t.Setenv("test_proxy", "http://proxy")

	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}
	if err := parse(t, fs, b); err != nil {
		t.Fatal(err)
	}

	if cfg.Str != "hello" {
		t.Errorf("derived env vars should be disabled without a prefix: %q", cfg.Str)
	}
	if cfg.ProxyEnvOnly != "http://proxy" {
		t.Errorf("explicit env tags still apply without a prefix: %q", cfg.ProxyEnvOnly)
	}
}

func TestSliceChoiceValidation(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	if err := parse(t, fs, b, "--slice-choice", "x", "--slice-choice", "y"); err != nil {
		t.Fatal(err)
	}

	fs2 := pflag.NewFlagSet("test", pflag.ContinueOnError)
	b2 := binder.NewRegistry("TESTPREFIX_").Binder(fs2)
	cfg2 := &kitchenSink{}
	if err := b2.Bind(cfg2, ""); err != nil {
		t.Fatal(err)
	}
	if err := parse(t, fs2, b2, "--slice-choice", "bogus"); err == nil {
		t.Error("slice elements should pass choice validation")
	}
}

func TestDefaultShownInUsage(t *testing.T) {
	cfg := &kitchenSink{}
	_, b, fs := newBinder(t, "TESTPREFIX_")
	if err := b.Bind(cfg, ""); err != nil {
		t.Fatal(err)
	}

	usage := fs.FlagUsages()
	if !strings.Contains(usage, "(default hello)") {
		t.Errorf("defaults should appear in usage:\n%s", usage)
	}
	if strings.Contains(usage, "sneaky") {
		t.Errorf("hidden flags should not appear in usage:\n%s", usage)
	}
}
