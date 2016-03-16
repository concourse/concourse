package integration_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/onsi/gomega"
)

const TableDurationPrefix = "__table_duration__"

type TableDurationWithDelta struct {
	Duration time.Duration
	Delta    time.Duration
	Suffix   string
}

func ParseTableDuration(data string) (TableDurationWithDelta, bool) {
	if !strings.HasPrefix(data, TableDurationPrefix) {
		return TableDurationWithDelta{}, false
	}

	parts := strings.Split(data, ":")
	if len(parts) != 4 {
		return TableDurationWithDelta{}, false
	}

	duration, _ := time.ParseDuration(parts[1])
	delta, _ := time.ParseDuration(parts[2])

	return TableDurationWithDelta{
		Duration: duration,
		Delta:    delta,
		Suffix:   parts[3],
	}, true
}

func (d TableDurationWithDelta) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", TableDurationPrefix, d.Duration, d.Delta, d.Suffix)
}

func (d TableDurationWithDelta) MatchString(actual string) error {
	if !strings.HasSuffix(actual, d.Suffix) {
		return fmt.Errorf("expected %s to have suffix %s", actual, d.Suffix)
	}

	actualDuration, _ := time.ParseDuration(strings.TrimSuffix(actual, d.Suffix))
	matched, err := gomega.BeNumerically("~", d.Delta, d.Duration).Match(actualDuration)
	if err != nil {
		return err
	}

	if !matched {
		return fmt.Errorf("expected duration %s is not within delta %s of actual duration %s", d.Duration, d.Delta, actualDuration)
	}

	return nil
}
