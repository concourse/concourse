package kubernetes

import (
	"fmt"
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/handles"
	"github.com/concourse/concourse/atc/worker/kubernetes/backend"
	log "github.com/sirupsen/logrus"
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
	cr     db.ContainerRepository
	name   string
}

var (
	// default resource types
	// TODO male this configurable per-worker
	//
	resourceTypes = []atc.WorkerResourceType{
		{
			Type:  "registry-image",
			Image: "concourse/registry-image-resource",
		},
		{
			Type:  "git",
			Image: "concourse/git-resource",
		},
		{
			Type:  "mock",
			Image: "concourse/mock-resource",
		},
		{
			Type:  "time",
			Image: "concourse/time-resource",
		},
	}
)

func NewTarget(
	wf db.WorkerFactory,
	syncer handles.Syncer,
	be *backend.Backend,
	cr db.ContainerRepository,
	// should we make this registerable in a per-cluster manner? if so, how?
) *target {
	info := atc.Worker{
		BaggageclaimURL: "baggageclaim",
		GardenAddr:      "k8s",
		Name:            "k8s",
		Platform:        "linux",
		ResourceTypes:   resourceTypes,
		Tags:            nil,
	}

	return &target{
		name:   "k8s",
		be:     be,
		info:   info,
		syncer: syncer,
		wf:     wf,
		cr:     cr,
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
	sess := log.WithFields(log.Fields{
		"who":    "target",
		"action": "sync",
	})

	sess.Info("start")
	defer sess.Info("finished")

	// TODO - validate that this query is really working
	//
	containers, err := t.be.Containers(map[string]string{
		backend.LabelConcourseKey: backend.LabelConcourseValue,
	})
	if err != nil {
		return fmt.Errorf("containers: %w", err)
	}

	handles := make([]string, 0, len(containers))
	for _, container := range containers {
		handles = append(handles, container.Handle())
	}

	// "report" current state
	//
	err = t.syncer.Sync(handles, t.name)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// get back info about what we should destroy
	//
	handlesToDestroy, err := t.cr.FindDestroyingContainers(t.name)
	if err != nil {
		return fmt.Errorf("find destroying containers: %w", err)
	}

	// actually destroy
	//
	err = t.destroyHandles(handlesToDestroy)
	if err != nil {
		return fmt.Errorf("destroy handles: %w", err)
	}

	return nil
}

func (t target) destroyHandles(handles []string) error {
	for _, handle := range handles {
		err := t.be.Destroy(handle)
		if err != nil {
			return fmt.Errorf("delete %s: %w", handle, err)
		}
	}

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

				err = t.Sync()
				if err != nil {
					return fmt.Errorf("sync: %w", err)
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
