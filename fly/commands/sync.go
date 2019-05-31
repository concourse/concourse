package commands

import (
	"fmt"
	"runtime"
	"strconv"

	update "github.com/inconshreveable/go-update"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"

	"github.com/concourse/concourse/v5"
	"github.com/concourse/concourse/v5/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/v5/fly/rc"
	"github.com/concourse/concourse/v5/fly/ui"
)

type SyncCommand struct {
	Force bool `long:"force" short:"f" description:"Sync even if versions already match."`
}

func (command *SyncCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
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
