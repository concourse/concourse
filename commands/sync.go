package commands

import (
	"fmt"
	"runtime"
	"strconv"

	pb "gopkg.in/cheggaaa/pb.v1"

	update "github.com/inconshreveable/go-update"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type SyncCommand struct{}

func (command *SyncCommand) Execute(args []string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
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
	defer progressBar.FinishPrint("update successful!")
	r := body
	reader := progressBar.NewProxyReader(r)

	err = update.Apply(reader, update.Options{})
	if err != nil {
		displayhelpers.Failf("update failed: %s", err)
	}

	return nil
}
