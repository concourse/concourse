package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/flag/binder"
	"github.com/concourse/concourse/worker/land"
	"github.com/concourse/concourse/worker/retire"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const envPrefix = "CONCOURSE_"

func concourseCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "concourse",
		Version:       concourse.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetVersionTemplate("{{.Version}}\n")
	// go-flags accepted -v/--version on any subcommand, not just the root
	root.PersistentFlags().BoolP("version", "v", false, "Print the version of Concourse and exit")
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// read from the root's flag set: the worker command has its own
		// (hidden, string-valued) --version flag shadowing this one
		if version, _ := root.PersistentFlags().GetBool("version"); version {
			fmt.Println(concourse.Version)
			os.Exit(0)
		}
	}
	root.SetUsageFunc(rootUsage)
	root.CompletionOptions.HiddenDefaultCmd = true

	registry := binder.NewRegistry(envPrefix)

	root.AddCommand(
		webCommand(registry),
		workerCommand(registry),
		migrateCommand(registry),
		quickstartCommand(registry),
		landWorkerCommand(registry),
		retireWorkerCommand(registry),
		generateKeyCommand(registry),
	)

	root.RunE = func(cmd *cobra.Command, args []string) error {
		// running bare `concourse` is an error, as with go-flags
		var names []string
		for _, sub := range cmd.Commands() {
			if sub.IsAvailableCommand() {
				names = append(names, sub.Name())
			}
		}

		return fmt.Errorf("Please specify one command of: %s",
			strings.Join(names[:len(names)-1], ", ")+" or "+names[len(names)-1])
	}

	return root
}

func webCommand(registry *binder.Registry) *cobra.Command {
	web := &WebCommand{}

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Run the web UI and build scheduler.",
	}
	cmd.Flags().SortFlags = false

	b := registry.Binder(cmd.Flags())
	b.MustBind(web, "")
	web.RunCommand.BindDynamicFlags(b, "")
	web.LessenRequirements(b)

	cmd.SetUsageFunc(commandUsage(b))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := b.Finish(); err != nil {
			return err
		}
		return web.Execute(args)
	}

	return cmd
}

func workerCommand(registry *binder.Registry) *cobra.Command {
	worker := &workercmd.WorkerCommand{}

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run and register a worker.",
	}
	cmd.Flags().SortFlags = false

	b := registry.Binder(cmd.Flags())
	b.MustBind(worker, "")
	worker.LessenRequirements("", b)

	cmd.SetUsageFunc(commandUsage(b))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := b.Finish(); err != nil {
			return err
		}
		return worker.Execute(args)
	}

	return cmd
}

func quickstartCommand(registry *binder.Registry) *cobra.Command {
	quickstart := &QuickstartCommand{}

	cmd := &cobra.Command{
		Use:   "quickstart",
		Short: "Run both 'web' and 'worker' together, auto-wired. Not recommended for production.",
	}
	cmd.Flags().SortFlags = false

	b := registry.Binder(cmd.Flags())
	b.MustBind(quickstart, "")
	quickstart.WebCommand.RunCommand.BindDynamicFlags(b, "")
	quickstart.LessenRequirements(b)

	cmd.SetUsageFunc(commandUsage(b))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := b.Finish(); err != nil {
			return err
		}
		return quickstart.Execute(args)
	}

	return cmd
}

func migrateCommand(registry *binder.Registry) *cobra.Command {
	migration := &atccmd.Migration{}

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations.",
	}
	cmd.Flags().SortFlags = false

	b := registry.Binder(cmd.Flags())
	b.MustBind(migration, "")

	cmd.SetUsageFunc(commandUsage(b))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := b.Finish(); err != nil {
			return err
		}
		return migration.Execute(args)
	}

	return cmd
}

func landWorkerCommand(registry *binder.Registry) *cobra.Command {
	landWorker := &land.LandWorkerCommand{}

	cmd := &cobra.Command{
		Use:   "land-worker",
		Short: "Safely drain a worker's assignments for temporary downtime.",
	}
	cmd.Flags().SortFlags = false

	b := registry.Binder(cmd.Flags())
	b.MustBind(landWorker, "")

	cmd.SetUsageFunc(commandUsage(b))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := b.Finish(); err != nil {
			return err
		}
		return landWorker.Execute(args)
	}

	return cmd
}

func retireWorkerCommand(registry *binder.Registry) *cobra.Command {
	retireWorker := &retire.RetireWorkerCommand{}

	cmd := &cobra.Command{
		Use:   "retire-worker",
		Short: "Safely remove a worker from the cluster permanently.",
	}
	cmd.Flags().SortFlags = false

	b := registry.Binder(cmd.Flags())
	b.MustBind(retireWorker, "")

	cmd.SetUsageFunc(commandUsage(b))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := b.Finish(); err != nil {
			return err
		}
		return retireWorker.Execute(args)
	}

	return cmd
}

func generateKeyCommand(registry *binder.Registry) *cobra.Command {
	generateKey := &GenerateKeyCommand{}

	cmd := &cobra.Command{
		Use:   "generate-key",
		Short: "Generate RSA key for use with Concourse components.",
	}
	cmd.Flags().SortFlags = false

	b := registry.Binder(cmd.Flags())
	b.MustBind(generateKey, "")

	cmd.SetUsageFunc(commandUsage(b))

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := b.Finish(); err != nil {
			return err
		}
		return generateKey.Execute(args)
	}

	return cmd
}

// rootUsage renders `concourse --help` in the old go-flags layout.
func rootUsage(c *cobra.Command) error {
	root := c.Root()
	w := c.OutOrStderr()

	fmt.Fprintf(w, "Usage:\n  %s [OPTIONS] <command>\n", root.Name())

	fmt.Fprintf(w, "\nApplication Options:\n")
	fmt.Fprintf(w, "  -v, --version  Print the version of Concourse and exit\n")

	fmt.Fprintf(w, "\nHelp Options:\n")
	fmt.Fprintf(w, "  -h, --help     Show this help message\n")

	names := []string{}
	padding := 0
	for _, sub := range root.Commands() {
		if sub.IsAvailableCommand() {
			names = append(names, sub.Name())
			padding = max(padding, len(sub.Name()))
		}
	}
	sort.Strings(names)

	fmt.Fprintf(w, "\nAvailable commands:\n")
	for _, name := range names {
		sub, _, _ := root.Find([]string{name})
		fmt.Fprintf(w, "  %-*s  %s\n", padding, name, sub.Short)
	}

	return nil
}

// commandUsage renders a subcommand's help in the old go-flags layout:
// one global description column, group headings, `(default: x) [$ENV]`
// suffixes — wrapped to the real terminal width, and degrading to a
// readable stacked layout on narrow terminals where go-flags produced
// garbage.
func commandUsage(b *binder.Binder) func(*cobra.Command) error {
	return func(c *cobra.Command) error {
		w := c.OutOrStderr()

		fmt.Fprintf(w, "Usage:\n  %s [OPTIONS] %s [%s-OPTIONS]\n\n", c.Root().Name(), c.Name(), c.Name())

		b.WriteUsage(w, binder.UsageOptions{
			CommandName: c.Name(),
			Width:       usageWidth(),
			RootOptions: []binder.RootOption{
				{Section: "Application Options", Short: "v", Long: "version", Description: "Print the version of Concourse and exit"},
				{Section: "Help Options", Short: "h", Long: "help", Description: "Show this help message"},
			},
		})

		return nil
	}
}

func usageWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil && width > 0 {
		return width
	}

	if cols, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && cols > 0 {
		return cols
	}

	// unknown; the renderer falls back to go-flags' default of 80
	return 0
}
