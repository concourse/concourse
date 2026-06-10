package main

import (
	"fmt"
	"os"

	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/flag/binder"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func main() {
	root := &cobra.Command{
		Use:           "atc",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// no env prefix: the standalone atc binary never supported
	// CONCOURSE_* environment configuration
	registry := binder.NewRegistry("")

	runCommand := &atccmd.RunCommand{}
	run := &cobra.Command{
		Use:   "run",
		Short: "Run the ATC.",
	}
	run.Flags().SortFlags = false
	rb := registry.Binder(run.Flags())
	rb.MustBind(runCommand, "")
	runCommand.BindDynamicFlags(rb, "")
	run.SetUsageFunc(usageFunc(rb))
	run.RunE = func(cmd *cobra.Command, args []string) error {
		if err := rb.Finish(); err != nil {
			return err
		}
		return runCommand.Execute(args)
	}

	migration := &atccmd.Migration{}
	migrate := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations.",
	}
	migrate.Flags().SortFlags = false
	mb := registry.Binder(migrate.Flags())
	mb.MustBind(migration, "")
	migrate.SetUsageFunc(usageFunc(mb))
	migrate.RunE = func(cmd *cobra.Command, args []string) error {
		if err := mb.Finish(); err != nil {
			return err
		}
		return migration.Execute(args)
	}

	root.AddCommand(run, migrate)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func usageFunc(b *binder.Binder) func(*cobra.Command) error {
	return func(c *cobra.Command) error {
		w := c.OutOrStderr()

		fmt.Fprintf(w, "Usage:\n  %s [OPTIONS] %s [%s-OPTIONS]\n\n", c.Root().Name(), c.Name(), c.Name())

		width := 0
		if cols, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			width = cols
		}

		b.WriteUsage(w, binder.UsageOptions{
			CommandName: c.Name(),
			Width:       width,
			RootOptions: []binder.RootOption{
				{Section: "Help Options", Short: "h", Long: "help", Description: "Show this help message"},
			},
		})

		return nil
	}
}
