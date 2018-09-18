package displayhelpers

import (
	"encoding/json"
	"fmt"
)

func JsonPrint(jsonObj interface{}) error {
	jsonBytes, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))
	return nil
}
