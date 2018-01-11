package main

import (
	"net/http"

	"github.com/concourse/skymarshal"
)

func main() {

	config := &skymarshal.Config{}

	server, err := skymarshal.NewHandler(config)
	if err != nil {
		panic(err)
	}

	http.ListenAndServe(":8081", server)
}
