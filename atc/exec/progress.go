package exec

import (
	"io"

	pb "gopkg.in/cheggaaa/pb.v1"
)

func progress(prefix string, stdout io.Writer) *pb.ProgressBar {
	bar := pb.New(0).SetUnits(pb.U_BYTES).Prefix(prefix)
	bar.Output = stdout
	bar.Width = 80
	bar.ForceWidth = true
	bar.ShowPercent = false
	bar.ShowCounters = false
	bar.ShowBar = false
	bar.ShowTimeLeft = false
	bar.ShowElapsedTime = false
	bar.ShowFinalTime = true
	bar.ShowSpeed = true
	bar.ShowTimeLeft = false

	return bar
}
