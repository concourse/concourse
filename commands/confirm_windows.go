// +build windows

package commands

import (
	"fmt"
	"log"
	"os"
)

func askToConfirm() bool {
	fmt.Printf("apply configuration? (y/n): ")

	var response string
	_, err := fmt.Fscanf(os.Stdin, "%s\n", &response)
	if err != nil {
		log.Fatalln("failed to read response:", err)
	}

	return response == "y"
}
