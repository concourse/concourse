package commands

import (
	"fmt"
	"runtime"
	"strconv"

	pb "gopkg.in/cheggaaa/pb.v1"

	"github.com/concourse/fly/version"
	update "github.com/inconshreveable/go-update"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type SyncCommand struct{}

func (command *SyncCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}
	info, err := target.Client().GetInfo()
	if err != nil {
		return err
	}

	if info.Version == version.Version {
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

	fmt.Printf("downloading fly from %s... \n", client.URL())

	filesSize, _ := strconv.ParseInt(headers.Get("Content-Length"), 10, 64)
	progressBar := pb.New64(filesSize).SetUnits(pb.U_BYTES)
	progressBar.Start()
	defer progressBar.FinishPrint(fmt.Sprintf("successfully updated from %s to %s", version.Version, info.Version))
	r := body
	reader := progressBar.NewProxyReader(r)

	err = update.Apply(reader, updateOptions)
	if err != nil {
		displayhelpers.FailWithErrorf("update failed", err)
	}

	return nil
}
