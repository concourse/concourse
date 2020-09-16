package commands

import (
	"fmt"
	"runtime"
	"strconv"

	update "github.com/inconshreveable/go-update"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"

	"github.com/concourse/concourse"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
)

type SyncCommand struct {
	Force          bool         `long:"force" short:"f" description:"Sync even if versions already match."`
	ATCURL         string       `long:"concourse-url" short:"c" description:"Concourse URL to sync with"`
	Insecure       bool         `short:"k" long:"insecure" description:"Skip verification of the endpoint's SSL certificate"`
	CACert         atc.PathFlag `long:"ca-cert" description:"Path to Concourse PEM-encoded CA certificate file."`
	ClientCertPath atc.PathFlag `long:"client-cert" description:"Path to a PEM-encoded client certificate file."`
	ClientKeyPath  atc.PathFlag `long:"client-key" description:"Path to a PEM-encoded client key file."`
}

func (command *SyncCommand) Execute(args []string) error {
	var target rc.Target
	var err error

	if Fly.Target != "" {
		target, err = rc.LoadTarget(Fly.Target, Fly.Verbose)
	} else {
		target, err = rc.NewUnauthenticatedTarget(
			"dummy",
			command.ATCURL,
			"",
			command.Insecure,
			string(command.CACert),
			string(command.ClientCertPath),
			string(command.ClientKeyPath),
			Fly.Verbose,
		)
	}
	if err != nil {
		return err
	}

	info, err := target.Client().GetInfo()
	if err != nil {
		return err
	}

	if !command.Force && info.Version == concourse.Version {
		fmt.Printf("version %s already matches; skipping\n", info.Version)
		return nil
	}

	updateOptions := update.Options{}
	err = updateOptions.CheckPermissions()
	if err != nil {
		displayhelpers.FailWithErrorf("update failed", err)
	}

	client := target.Client()
	body, headers, err := client.GetCLIReader(runtime.GOARCH, runtime.GOOS)
	if err != nil {
		return err
	}

	fmt.Printf("downloading fly from %s...\n", client.URL())
	fmt.Println()

	size, err := strconv.ParseInt(headers.Get("Content-Length"), 10, 64)
	if err != nil {
		fmt.Printf("warning: failed to parse Content-Length: %s\n", err)
		size = 0
	}

	progress := mpb.New(mpb.WithWidth(50))

	progressBar := progress.AddBar(
		size,
		mpb.PrependDecorators(decor.Name("fly "+ui.Embolden("v"+info.Version))),
		mpb.AppendDecorators(decor.CountersKibiByte("%.1f/%.1f")),
	)

	err = update.Apply(progressBar.ProxyReader(body), updateOptions)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to apply update", err)
	}

	if size == 0 {
		progressBar.SetTotal(progressBar.Current(), true)
	}

	progress.Wait()

	fmt.Println("done")

	return nil
}
