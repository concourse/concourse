package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/concourse/atc"
)

func main() {
	workerJSON, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	var worker atc.Worker
	err = json.Unmarshal(workerJSON, &worker)
	if err != nil {
		log.Fatal(err)
	}

	if worker.ResourceTypes == nil {
		worker.ResourceTypes = []atc.WorkerResourceType{}
	}

	for _, file := range os.Args[1:] {
		metadataJSON, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}

		var workerResourceType atc.WorkerResourceType
		err = json.Unmarshal(metadataJSON, &workerResourceType)
		if err != nil {
			log.Fatal(err)
		}

		worker.ResourceTypes = append(worker.ResourceTypes, workerResourceType)
	}

	updatedJSON, err := json.Marshal(worker)
	if err != nil {
		log.Fatal(err)
	}

	os.Stdout.Write(updatedJSON)
}
