package stream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/vito/go-sse/sse"
)

type WriteFlusher interface {
	io.Writer
	http.Flusher
}

type EventWriter struct {
	WriteFlusher
}

func (writer EventWriter) WriteEvent(id uint, name string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = sse.Event{
		ID:   fmt.Sprintf("%d", id),
		Name: name,
		Data: payload,
	}.Write(writer)
	if err != nil {
		return err
	}

	writer.Flush()

	return nil
}

func (writer EventWriter) WriteEnd(id uint) error {
	err := sse.Event{
		ID:   fmt.Sprintf("%d", id),
		Name: "end",
	}.Write(writer)
	if err != nil {
		return err
	}

	writer.Flush()

	return nil
}
