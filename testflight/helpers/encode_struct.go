package helpers

import (
	"bytes"
	"encoding/gob"
)

func EncodeStruct(obj interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	err := gob.NewEncoder(&buffer).Encode(obj)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func DecodeStruct(data []byte, ret interface{}) error {
	buffer := bytes.NewBuffer(data)
	err := gob.NewDecoder(buffer).Decode(ret)
	if err != nil {
		return err
	}
	return nil
}
