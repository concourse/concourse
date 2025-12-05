package engine

import (
	"errors"
	"testing"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBEventWriter(t *testing.T) {
	setup := func() (*dbfakes.FakeBuild, event.Origin, *fakeclock.FakeClock) {
		fakeBuild := new(dbfakes.FakeBuild)
		origin := event.Origin{ID: "some-id"}
		fakeClock := fakeclock.NewFakeClock(time.Unix(1000, 0))
		return fakeBuild, origin, fakeClock
	}

	t.Run("writes complete UTF-8 text immediately", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup()
		writer := newDBEventWriter(fakeBuild, origin, fakeClock)

		n, err := writer.Write([]byte("hello world"))
		require.NoError(t, err)
		assert.Equal(t, 11, n)

		require.Equal(t, 1, fakeBuild.SaveEventCallCount())
		savedEvent := fakeBuild.SaveEventArgsForCall(0)
		logEvent := savedEvent.(event.Log)
		assert.Equal(t, "hello world", logEvent.Payload)
		assert.Equal(t, int64(1000), logEvent.Time)
		assert.Equal(t, origin, logEvent.Origin)
	})

	t.Run("buffers incomplete UTF-8 sequence", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup()
		writer := newDBEventWriter(fakeBuild, origin, fakeClock)

		// UTF-8 for "日" is 0xE6 0x97 0xA5 (3 bytes)
		// Send first two bytes - incomplete
		n, err := writer.Write([]byte{0xE6, 0x97})
		require.NoError(t, err)
		assert.Equal(t, 2, n)
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount(), "should not save incomplete UTF-8")

		// Send final byte
		n, err = writer.Write([]byte{0xA5})
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "日", logEvent.Payload)
	})

	t.Run("buffers multi-byte character split across three writes", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup()
		writer := newDBEventWriter(fakeBuild, origin, fakeClock)

		// 4-byte UTF-8: 𝄞 (musical G clef) = F0 9D 84 9E
		writer.Write([]byte{0xF0})
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount())

		writer.Write([]byte{0x9D})
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount())

		writer.Write([]byte{0x84, 0x9E})
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "𝄞", logEvent.Payload)
	})

	t.Run("Close flushes dangling bytes", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup()
		writer := newDBEventWriter(fakeBuild, origin, fakeClock)

		// Write incomplete UTF-8
		writer.Write([]byte{0xE6, 0x97})
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount())

		err := writer.Close()
		require.NoError(t, err)
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())
	})

	t.Run("Close with no dangling does nothing", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup()
		writer := newDBEventWriter(fakeBuild, origin, fakeClock)

		err := writer.Close()
		require.NoError(t, err)
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount())
	})

	t.Run("propagates SaveEvent error", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup()
		writer := newDBEventWriter(fakeBuild, origin, fakeClock)

		expectedErr := errors.New("db error")
		fakeBuild.SaveEventReturns(expectedErr)

		n, err := writer.Write([]byte("hello"))
		assert.Equal(t, 0, n)
		assert.Equal(t, expectedErr, err)
	})
}

func TestDBEventWriterWithSecretRedaction(t *testing.T) {
	setup := func(filter func(string) string) (*dbfakes.FakeBuild, event.Origin, *fakeclock.FakeClock) {
		fakeBuild := new(dbfakes.FakeBuild)
		origin := event.Origin{ID: "some-id"}
		fakeClock := fakeclock.NewFakeClock(time.Unix(1000, 0))
		return fakeBuild, origin, fakeClock
	}

	passthrough := func(s string) string { return s }

	t.Run("buffers until newline", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, passthrough)

		// Write without newline - should buffer
		n, err := writer.Write([]byte("no newline yet"))
		require.NoError(t, err)
		assert.Equal(t, 14, n)
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount(), "should buffer until newline")

		// Write with newline - should flush
		n, err = writer.Write([]byte(" now newline\n"))
		require.NoError(t, err)
		assert.Equal(t, 13, n)
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "no newline yet now newline\n", logEvent.Payload)
	})

	t.Run("keeps content after last newline in buffer", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, passthrough)

		writer.Write([]byte("line1\npartial"))
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "line1\n", logEvent.Payload)

		// Complete the partial line
		writer.Write([]byte(" line2\n"))
		require.Equal(t, 2, fakeBuild.SaveEventCallCount())

		logEvent = fakeBuild.SaveEventArgsForCall(1).(event.Log)
		assert.Equal(t, "partial line2\n", logEvent.Payload)
	})

	t.Run("applies filter to payload", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		redactor := func(s string) string {
			return "[REDACTED]\n"
		}
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, redactor)

		writer.Write([]byte("secret-password-123\n"))
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "[REDACTED]\n", logEvent.Payload)
	})

	t.Run("Close flushes remaining content without newline", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, passthrough)

		writer.Write([]byte("partial without newline"))
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount())

		writer.(interface{ Close() error }).Close()
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "partial without newline", logEvent.Payload)
	})

	t.Run("Close with empty buffer does nothing", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, passthrough)

		writer.(interface{ Close() error }).Close()
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount())
	})

	t.Run("handles multiple newlines in single write", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, passthrough)

		writer.Write([]byte("line1\nline2\nline3\npartial"))
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "line1\nline2\nline3\n", logEvent.Payload)

		// Flush partial
		writer.(interface{ Close() error }).Close()
		require.Equal(t, 2, fakeBuild.SaveEventCallCount())

		logEvent = fakeBuild.SaveEventArgsForCall(1).(event.Log)
		assert.Equal(t, "partial", logEvent.Payload)
	})

	t.Run("buffers incomplete UTF-8 before checking newlines", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, passthrough)

		// "日\n" in UTF-8: E6 97 A5 0A
		// Send incomplete
		writer.Write([]byte{0xE6, 0x97})
		assert.Equal(t, 0, fakeBuild.SaveEventCallCount())

		// Complete UTF-8 + newline
		writer.Write([]byte{0xA5, 0x0A})
		require.Equal(t, 1, fakeBuild.SaveEventCallCount())

		logEvent := fakeBuild.SaveEventArgsForCall(0).(event.Log)
		assert.Equal(t, "日\n", logEvent.Payload)
	})

	t.Run("propagates SaveEvent error", func(t *testing.T) {
		fakeBuild, origin, fakeClock := setup(passthrough)
		writer := newDBEventWriterWithSecretRedaction(fakeBuild, origin, fakeClock, passthrough)

		expectedErr := errors.New("db error")
		fakeBuild.SaveEventReturns(expectedErr)

		n, err := writer.Write([]byte("hello\n"))
		assert.Equal(t, 0, n)
		assert.Equal(t, expectedErr, err)
	})
}
