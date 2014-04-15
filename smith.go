package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pivotal-golang/archiver/compressor"
)

func main() {
	compressor := compressor.NewTgz()
	src, err := os.Getwd()
	if err != nil {
		fmt.Println("Couldn't get current directory...")
		os.Exit(1)
	}
	dest, err := ioutil.TempFile("", "smith")
	if err != nil {
		fmt.Println("Couldn't create temporary file...")
		os.Exit(1)
	}
	dest.Close()

	err = compressor.Compress(src, dest.Name())
	if err != nil {
		fmt.Printf("Couldn't create archive: %s\n", err.Error())
		os.Exit(1)
	}

	os.Rename(dest.Name(), "dir.tgz")

	fmt.Printf("Created archive of current directory.")
}
