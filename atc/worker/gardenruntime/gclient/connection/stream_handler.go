package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"code.cloudfoundry.org/garden/transport"
	"code.cloudfoundry.org/lager"
)

type streamHandler struct {
	log lager.Logger
	wg  *sync.WaitGroup
}

func newStreamHandler(log lager.Logger) *streamHandler {
	return &streamHandler{
		log: log,
		wg:  new(sync.WaitGroup),
	}
}

func (sh *streamHandler) streamIn(processWriter io.WriteCloser, stdin io.Reader) {
	if stdin == nil {
		return
	}

	go func(processInputStream io.WriteCloser, stdin io.Reader, log lager.Logger) {
		if _, err := io.Copy(processInputStream, stdin); err == nil {
			processInputStream.Close()
		} else {
			log.Error("streaming-stdin-payload", err)
		}
	}(processWriter, stdin, sh.log)
}

func (sh *streamHandler) streamOut(streamWriter io.Writer, streamReader io.Reader) {
	sh.wg.Add(1)
	go func() {
		io.Copy(streamWriter, streamReader)
		sh.wg.Done()
	}()
}

type waitReturn struct {
	exitCode int
	err      error
}

func (sh *streamHandler) wait(decoder *json.Decoder) <-chan waitReturn {
	result := make(chan waitReturn)
	go func() {
		for {
			payload := &transport.ProcessPayload{}
			err := decoder.Decode(payload)
			if err != nil {
				sh.wg.Wait()
				result <- waitReturn{
					0,
					fmt.Errorf("connection: decode failed: %s", err),
				}
				break
			}

			if payload.Error != nil {
				sh.wg.Wait()
				result <- waitReturn{
					0,
					fmt.Errorf("connection: process error: %s", *payload.Error),
				}
				break
			}

			if payload.ExitStatus != nil {
				sh.wg.Wait()
				result <- waitReturn{
					int(*payload.ExitStatus),
					nil,
				}
				break
			}

			// discard other payloads
		}
		defer close(result)
	}()
	return result
}
