package main

import (
	"net/http"

	"github.com/concourse/concourse/skymarshal"
)

func main() {

	config := &skymarshal.Config{}

	server, err := skymarshal.NewServer(config)
	if err != nil {
		panic(err)
	}

	http.ListenAndServe(":8081", server)
}
