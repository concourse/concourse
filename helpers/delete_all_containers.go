package helpers

import (
	gclient "code.cloudfoundry.org/garden/client"
	gconn "code.cloudfoundry.org/garden/client/connection"
	"github.com/concourse/go-concourse/concourse"
)

func DeleteAllContainers(client concourse.Client, name string) error {
	workers, err := client.ListWorkers()
	if err != nil {
		return err
	}

	containers, err := client.ListContainers(map[string]string{"pipeline_name": name})
	if err != nil {
		return err
	}

	for _, worker := range workers {
		connection := gconn.New("tcp", worker.GardenAddr)
		gardenClient := gclient.New(connection)
		for _, container := range containers {
			if container.WorkerName == worker.Name {
				err = gardenClient.Destroy(container.ID)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
