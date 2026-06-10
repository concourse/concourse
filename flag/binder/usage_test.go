package binder_test

import (
	"strings"
	"testing"

	"github.com/concourse/concourse/flag/binder"
	"github.com/spf13/pflag"
)

type usageCommand struct {
	PeerAddress  string `long:"peer-address" default:"127.0.0.1" description:"Network address of this node."`
	Quiet        bool   `long:"quiet" description:"Say less."`
	Undocumented string `long:"undocumented"`

	Server struct {
		XFrameOptions string `long:"x-frame-options" default:"deny" description:"The value to set for the X-Frame-Options header."`
		ClientSecret  string `long:"client-secret" required:"true" description:"Client secret to use for login flow"`
	} `group:"Web Server"`

	Auth struct {
		PasswordConnector string            `long:"password-connector" default:"local" choice:"local" choice:"ldap" description:"Connector to use when authenticating"`
		LocalUsers        map[string]string `long:"add-local-user" description:"List of username:password combinations." value-name:"USERNAME:PASSWORD"`
	} `group:"Authentication"`

	Sneaky bool `long:"sneaky" hidden:"true" description:"Not shown."`
}

func renderUsage(t *testing.T, width int) string {
	t.Helper()

	fs := pflag.NewFlagSet("web", pflag.ContinueOnError)
	b := binder.NewRegistry("CONCOURSE_").Binder(fs)
	if err := b.Bind(&usageCommand{}, ""); err != nil {
		t.Fatal(err)
	}
	b.MustBindGroup("Metric Emitter (InfluxDB)", &struct {
		URL string `long:"influxdb-url" description:"InfluxDB server address to emit points to."`
	}{}, "")

	var out strings.Builder
	b.WriteUsage(&out, binder.UsageOptions{
		CommandName: "web",
		Width:       width,
		RootOptions: []binder.RootOption{
			{Section: "Application Options", Short: "v", Long: "version", Description: "Print the version of Concourse and exit"},
			{Section: "Help Options", Short: "h", Long: "help", Description: "Show this help message"},
		},
	})
	return out.String()
}

func TestUsageGroupHeadingsAndOrder(t *testing.T) {
	out := renderUsage(t, 200)

	wantOrder := []string{
		"Application Options:",
		"-v, --version",
		"Help Options:",
		"-h, --help",
		"[web command options]",
		"--peer-address=",
		"    Web Server:",
		"--x-frame-options=",
		"    Authentication:",
		"--password-connector=[local|ldap]",
		"    Metric Emitter (InfluxDB):",
		"--influxdb-url=",
	}

	pos := 0
	for _, want := range wantOrder {
		idx := strings.Index(out[pos:], want)
		if idx < 0 {
			t.Fatalf("expected %q after position %d in:\n%s", want, pos, out)
		}
		pos += idx
	}
}

func TestUsageEnvAndDefaultSuffixes(t *testing.T) {
	out := renderUsage(t, 200)

	for _, want := range []string{
		"Network address of this node. (default: 127.0.0.1) [$CONCOURSE_PEER_ADDRESS]",
		"The value to set for the X-Frame-Options header. (default: deny) [$CONCOURSE_X_FRAME_OPTIONS]",
		"Connector to use when authenticating (default: local) [$CONCOURSE_PASSWORD_CONNECTOR]",
		"List of username:password combinations. [$CONCOURSE_ADD_LOCAL_USER]",
		"--add-local-user=USERNAME:PASSWORD",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestUsageBooleansTakeNoArgument(t *testing.T) {
	out := renderUsage(t, 200)

	if !strings.Contains(out, "--quiet ") && !strings.Contains(out, "--quiet\n") {
		t.Errorf("bool flag should have no = suffix:\n%s", out)
	}
	if strings.Contains(out, "--quiet=") {
		t.Errorf("bool flag must not render =:\n%s", out)
	}
}

func TestUsageHiddenAndUndocumented(t *testing.T) {
	out := renderUsage(t, 200)

	if strings.Contains(out, "sneaky") {
		t.Errorf("hidden flags must not render:\n%s", out)
	}

	// flags without a description get no env/default suffix, like go-flags
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "--undocumented") && strings.Contains(line, "$CONCOURSE_UNDOCUMENTED") {
			t.Errorf("undocumented flag should have a bare line: %q", line)
		}
	}
}

func TestUsageGlobalAlignment(t *testing.T) {
	out := renderUsage(t, 500)

	// at a generous width every description starts in the same column
	col := -1
	for _, probe := range []string{"Network address", "The value to set", "Connector to use", "InfluxDB server"} {
		for _, line := range strings.Split(out, "\n") {
			idx := strings.Index(line, probe)
			if idx < 0 {
				continue
			}
			if col == -1 {
				col = idx
			} else if idx != col {
				t.Errorf("description %q starts at %d, want global column %d:\n%s", probe, idx, col, out)
			}
		}
	}
	if col == -1 {
		t.Fatal("no descriptions found")
	}
}

func TestUsageWrapsToWidth(t *testing.T) {
	out := renderUsage(t, 100)

	for _, line := range strings.Split(out, "\n") {
		if len(line) > 100 {
			t.Errorf("line longer than width 100: %q", line)
		}
	}
}

func TestUsageNarrowTerminalClamp(t *testing.T) {
	out := renderUsage(t, 50)

	// descriptions move to their own indented lines and nothing exceeds
	// a readable layout
	if !strings.Contains(out, "\n        Network address of this node.") {
		t.Errorf("narrow layout should put descriptions on their own lines:\n%s", out)
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Network") && len(line) > 50 {
			t.Errorf("narrow description line exceeds width: %q", line)
		}
	}
}
