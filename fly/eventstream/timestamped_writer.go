package eventstream

import (
	"io"
	"sync"
	"time"
)

const (
	// Pre-defined timestamp length (HH:MM:SS + two spaces)
	timestampLen = 10

	// Buffer for newline counting
	maxNewlines = 100
)

// Re-usable buffer pool to reduce allocations
var bufferPool = sync.Pool{
	New: func() any {
		// Default size that should handle most log lines
		buf := make([]byte, 0, 256)
		return &buf
	},
}

type TimestampedWriter struct {
	showTimestamp bool
	time          []byte // Current timestamp
	writer        io.Writer
	newLine       bool
	newlineCount  int   // Track estimated newlines for buffer sizing
	newlineBuf    []int // Reusable buffer for newline positions
}

func (w *TimestampedWriter) Write(b []byte) (int, error) {
	// Fast path for no timestamps
	if !w.showTimestamp {
		return w.writer.Write(b)
	}

	// Fast path for empty input
	if len(b) == 0 {
		return 0, nil
	}

	// Get buffer from pool
	bufPtr := bufferPool.Get().(*[]byte)
	toWrite := (*bufPtr)[:0] // Reset length but keep capacity

	// Count newlines and compute needed capacity
	w.newlineCount = 0
	if w.newlineBuf == nil {
		w.newlineBuf = make([]int, maxNewlines)
	}

	for i, c := range b {
		if c == '\n' && w.newlineCount < maxNewlines {
			w.newlineBuf[w.newlineCount] = i
			w.newlineCount++
		}
	}

	// Calculate initial capacity - current bytes + potential timestamps
	neededCap := len(b) + timestampLen
	if w.newlineCount > 0 {
		// Add space for a timestamp after each newline (except the last one)
		neededCap += timestampLen * w.newlineCount
	}

	// Ensure buffer has enough capacity
	if cap(toWrite) < neededCap {
		// Create new buffer with sufficient capacity
		*bufPtr = make([]byte, 0, neededCap*2) // Double for future growth
		toWrite = *bufPtr
	}

	// Add initial timestamp if at start of line
	if w.newLine {
		toWrite = append(toWrite, w.time...)
	}

	// Process input by segments between newlines for better performance
	lastPos := 0
	for i := 0; i < w.newlineCount; i++ {
		nlPos := w.newlineBuf[i]

		// Add segment up to and including newline
		toWrite = append(toWrite, b[lastPos:nlPos+1]...)

		// Add timestamp after newline if not the last byte
		if nlPos < len(b)-1 {
			toWrite = append(toWrite, w.time...)
		}

		lastPos = nlPos + 1
	}

	// Add any remaining content
	if lastPos < len(b) {
		toWrite = append(toWrite, b[lastPos:]...)
	}

	// Set newLine state based on whether the last char was a newline
	if len(b) > 0 {
		w.newLine = b[len(b)-1] == '\n'
	}

	// Write to the underlying writer
	_, err := w.writer.Write(toWrite)

	// Update the pool buffer with our changes
	*bufPtr = toWrite
	bufferPool.Put(bufPtr)

	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func NewTimestampedWriter(writer io.Writer, showTime bool) *TimestampedWriter {
	tw := &TimestampedWriter{
		showTimestamp: showTime,
		writer:        writer,
		newLine:       showTime,
	}

	// Pre-allocate timestamp buffer
	if showTime {
		tw.time = make([]byte, timestampLen)
		for i := range tw.time {
			tw.time[i] = ' ' // Default to spaces
		}
	}

	return tw
}

func (w *TimestampedWriter) SetTimestamp(timestamp int64) {
	if !w.showTimestamp {
		return
	}

	// Ensure timestamp buffer exists
	if w.time == nil || len(w.time) != timestampLen {
		w.time = make([]byte, timestampLen)
	}

	if timestamp == 0 {
		// Fill with spaces for empty timestamp
		for i := range w.time {
			w.time[i] = ' '
		}
		return
	}

	// Manual formatting is faster than time.Format
	t := time.Unix(timestamp, 0)
	h, m, s := t.Hour(), t.Minute(), t.Second()

	// Write directly to the buffer
	w.time[0] = '0' + byte(h/10)
	w.time[1] = '0' + byte(h%10)
	w.time[2] = ':'
	w.time[3] = '0' + byte(m/10)
	w.time[4] = '0' + byte(m%10)
	w.time[5] = ':'
	w.time[6] = '0' + byte(s/10)
	w.time[7] = '0' + byte(s%10)
	// Last two bytes are spaces
	w.time[8] = ' '
	w.time[9] = ' '
}
