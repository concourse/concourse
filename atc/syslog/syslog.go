package syslog

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	sl "github.com/racksec/srslog"
)

const rfc5424time = "2006-01-02T15:04:05.999999Z07:00"
const priority = sl.LOG_USER | sl.LOG_INFO

var replacer = strings.NewReplacer("\n", " ", "\r", " ", "\x00", " ")

type Syslog struct {
	writer *sl.Writer
	mu     sync.RWMutex
}

func Dial(transport, address string, caCerts []string) (*Syslog, error) {
	var (
		syslog *sl.Writer
		config *tls.Config = nil
	)

	if transport == "tls" {
		certpool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("failed to get system cert pool: %w", err)
		}

		for _, cert := range caCerts {
			content, err := os.ReadFile(cert)
			if err != nil {
				return nil, fmt.Errorf("failed to read certificate file %s: %w", cert, err)
			}

			ok := certpool.AppendCertsFromPEM(content)
			if !ok {
				return nil, fmt.Errorf("failed to parse certificate from file %s", cert)
			}
		}
		// srslog uses "tcp+tls" to specify "tls" connections
		transport = "tcp+tls"

		config = &tls.Config{
			RootCAs:    certpool,
			MinVersion: tls.VersionTLS12, // Enforce minimum TLS 1.2 for security
		}
	}

	syslog, err := sl.DialWithTLSConfig(transport, address, priority, "", config)
	if err != nil {
		return nil, err
	}

	return &Syslog{
		writer: syslog,
	}, nil
}

func (s *Syslog) Write(hostname, tag string, ts time.Time, msg string, eventID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.writer == nil {
		return errors.New("connection already closed")
	}

	// Set formatter once with all the parameters needed
	s.writer.SetFormatter(getSyslogFormatter(hostname, ts, tag, eventID))

	// Avoid unnecessary allocation if msg is already a string
	_, err := s.writer.Write([]byte(msg))
	return err
}

func (s *Syslog) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.writer == nil {
		return errors.New("connection already closed")
	}

	err := s.writer.Close()
	if err == nil {
		s.writer = nil
	}

	return err
}

// generate custom formatter based on hostname and tag
func getSyslogFormatter(hostname string, ts time.Time, tag string, eventID string) sl.Formatter {
	timeStr := ts.Format(rfc5424time)

	return func(priority sl.Priority, _, _, content string) string {
		// Strip whitespaces using the pre-defined replacer
		s := replacer.Replace(content)

		msg := fmt.Sprintf("<%d>1 %s %s %s - - [concourse@0 eventId=\"%s\"] %s\n",
			priority, timeStr, hostname, tag, eventID, s)
		return msg
	}
}
