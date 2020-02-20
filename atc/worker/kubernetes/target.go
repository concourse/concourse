package kubernetes

import (
	"fmt"
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/handles"
	"github.com/concourse/concourse/atc/worker/kubernetes/backend"
	"github.com/tedsuo/ifrit"
)

type Target interface {
	Retire() error
	Heartbeat() error
	Sync() error
}

type target struct {
	wf     db.WorkerFactory
	syncer handles.Syncer
	info   atc.Worker
	be     *backend.Backend
}

var (
	resourceTypes = map[string]string{}
)

func NewTarget(
	wf db.WorkerFactory,
	syncer handles.Syncer,
	be *backend.Backend,
	// should we make this registerable in a per-cluster manner? if so, how?
) *target {
	info := atc.Worker{
		BaggageclaimURL: "baggageclaim",
		GardenAddr:      "k8s",
		Name:            "k8s",
		Platform:        "linux",
		ResourceTypes: []atc.WorkerResourceType{
			{
				Type:  "registry-image",
				Image: "concourse/registry-image-resource",
			},
			{
				Type:  "git",
				Image: "concourse/git-resource",
			},
		}, // TODO get this configured
		Tags: nil,
	}

	return &target{
		be:     be,
		info:   info,
		syncer: syncer,
		wf:     wf,
	}
}

func (t target) Heartbeat() error {
	ttl := 30 * time.Second

	_, err := t.wf.SaveWorker(t.info, ttl)
	if err != nil {
		return fmt.Errorf("save worker: %w", err)
	}

	return nil
}

func (t target) Retire() error {
	return nil
}

func (t target) Sync() error {

	// retrieve handles (backend `list` with properties)
	// syncer.Sync(handles, w.info.name)
	return nil
}

func NewTargetRunner(t Target) ifrit.RunFunc {
	return func(signals <-chan os.Signal, ready chan<- struct{}) error {

		ticker := time.NewTicker(10 * time.Second)
		close(ready) // is this right?

	loop:
		for {
			select {
			case <-ticker.C:
				err := t.Heartbeat()
				if err != nil {
					return fmt.Errorf("target heartbeat: %w", err)
				}
			case <-signals:
				err := t.Retire()
				if err != nil {
					return fmt.Errorf("target retire: %w", err)
				}
				break loop
			}
		}

		return nil
	}
}
