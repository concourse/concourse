package postgresrunner_test

import (
	"github.com/concourse/concourse/atc/postgresrunner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExtractQueries", func() {
	It("parses queries out of a top-level query", func() {
		query := `WITH abc AS (
  SELECT def FROM (SELECT 1)
), ghi AS (
  SELECT blah
  UNION
  SELECT bloo
)
SELECT who, cares
FROM something
JOIN other ON col1 = (SELECT other_col FROM something)
	union
SELECT something_else FROM a_table`

		Expect(postgresrunner.ExtractQueries(query)).To(ConsistOf(
			`WITH abc AS (...), ghi AS (...)
SELECT who, cares
FROM something
JOIN other ON col1 = (...)`,
			`SELECT something_else FROM a_table`,
			`SELECT other_col FROM something`,
			`SELECT def FROM (...)`,
			`SELECT blah`,
			`SELECT bloo`,
			`SELECT 1`,
		))
	})
})
