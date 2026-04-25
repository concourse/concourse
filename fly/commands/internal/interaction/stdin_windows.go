package interaction

import (
	"io"
	"os"
	"sync"
)

var (
	stdinPumpOnce sync.Once
	stdinBufCh    chan []byte

	activeMu      sync.Mutex
	activeWrapper *wrappedReader
)

func startStdinPump() {
	stdinPumpOnce.Do(func() {
		stdinBufCh = make(chan []byte, 16)
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := os.Stdin.Read(buf)
				if n > 0 {
					chunk := make([]byte, n)
					copy(chunk, buf[:n])
					stdinBufCh <- chunk
				}
				if err != nil {
					close(stdinBufCh)
					return
				}
			}
		}()
	})
}

// Stdin returns a reader that drains the process's stdin via a singleton pump
// goroutine and a shared channel. Each call closes the previously returned
// reader so any pending Read on it returns EOF without losing data — the
// queued bytes stay in the channel for the next consumer.
//
// On Windows the charm libraries detect an *os.File and read CONIN$ directly,
// which bypasses gexec-redirected stdin in tests. Wrapping os.Stdin in a
// non-File reader forces the cancelreader fallback path. But that fallback
// can't actually interrupt a Read syscall, so a bubbletea program leaves
// behind a stale read goroutine after Run() returns; without the auto-close
// dance below it would consume (and discard) input meant for the next
// consumer — e.g. the io.Copy in `fly hijack` after a container selection.
func Stdin() io.Reader {
	if os.Getenv("FLY_TEST") == "" {
		return os.Stdin
	}

	activeMu.Lock()
	defer activeMu.Unlock()

	if activeWrapper != nil {
		activeWrapper.close()
	}

	startStdinPump()
	activeWrapper = &wrappedReader{
		bufCh:   stdinBufCh,
		closeCh: make(chan struct{}),
	}
	return activeWrapper
}

type wrappedReader struct {
	bufCh     chan []byte
	closeCh   chan struct{}
	closeOnce sync.Once
	leftover  []byte
}

func (r *wrappedReader) Read(data []byte) (int, error) {
	if len(r.leftover) > 0 {
		n := copy(data, r.leftover)
		r.leftover = r.leftover[n:]
		return n, nil
	}
	select {
	case <-r.closeCh:
		return 0, io.EOF
	case chunk, ok := <-r.bufCh:
		if !ok {
			return 0, io.EOF
		}
		n := copy(data, chunk)
		if n < len(chunk) {
			r.leftover = chunk[n:]
		}
		return n, nil
	}
}

func (r *wrappedReader) close() {
	r.closeOnce.Do(func() {
		close(r.closeCh)
	})
}
