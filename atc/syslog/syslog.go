package syslog

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	sl "github.com/racksec/srslog"
)

const rfc5424time = "2006-01-02T15:04:05.999999Z07:00"
const priority = sl.LOG_USER | sl.LOG_INFO

type Syslog struct {
	writer *sl.Writer
	closed bool

	mu sync.RWMutex
}

func Dial(transport, address string, caCerts []string) (*Syslog, error) {
	var (
		syslog *sl.Writer
		config *tls.Config = nil
	)

	if transport == "tls" {
		certpool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		for _, cert := range caCerts {
			content, err := ioutil.ReadFile(cert)
			if err != nil {
				return nil, err
			}

			ok := certpool.AppendCertsFromPEM(content)
			if !ok {
				return nil, errors.New("syslog drainer certificate error")
			}
		}
		// srslog uses "tcp+tls" to specify "tls" connections
		transport = "tcp+tls"

		config = &tls.Config{
			RootCAs: certpool,
		}
	}

	syslog, err := sl.DialWithTLSConfig(transport, address, priority, "", config)
	if err != nil {
		return nil, err
	}

	return &Syslog{
		writer: syslog,
		closed: false,
	}, nil
}

func (s *Syslog) Write(hostname, tag string, ts time.Time, msg string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.writer == nil {
		return errors.New("connection already closed")
	}

	s.writer.SetFormatter(getSyslogFormatter(hostname, ts, tag))
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
func getSyslogFormatter(hostname string, ts time.Time, tag string) sl.Formatter {
	return func(priority sl.Priority, _, _, content string) string {
		// strip whitespaces
		s := strings.Replace(content, "\n", " ", -1)
		s = strings.Replace(s, "\r", " ", -1)
		s = strings.Replace(s, "\x00", " ", -1)

		msg := fmt.Sprintf("<%d>1 %s %s %s - - - %s\n",
			priority, ts.Format(rfc5424time), hostname, tag, s)
		return msg
	}
}
