package progress

import (
	"fmt"

	"github.com/concourse/concourse/fly/ui"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"golang.org/x/sync/errgroup"
)

type Progress struct {
	progress *mpb.Progress
	errs     *errgroup.Group
}

func New() *Progress {
	return &Progress{
		progress: mpb.New(mpb.WithWidth(1)),
		errs:     new(errgroup.Group),
	}
}

func (prog *Progress) Go(name string, f func(*mpb.Bar) error) {
	bar := prog.progress.New(
		0,
		mpb.SpinnerStyle().PositionLeft(),
		mpb.PrependDecorators(
			decor.Name(
				name,
				decor.WC{W: len(name), C: decor.DSyncWidthR},
			),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.AverageSpeed(decor.SizeB1024(0), "(%.1f)"),
				" "+ui.Embolden("done"),
			),
		),
		mpb.BarFillerClearOnComplete(),
	)

	prog.errs.Go(func() error {
		err := f(bar)
		if err != nil {
			bar.Abort(false)
			return fmt.Errorf("'%s' failed: %s", name, err)
		}

		bar.SetTotal(bar.Current(), true)

		return nil
	})
}

func (prog *Progress) Wait() error {
	prog.progress.Wait()
	return prog.errs.Wait()
}
