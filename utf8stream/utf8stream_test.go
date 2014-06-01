package utf8stream_test

import (
	. "github.com/winston-ci/winston/utf8stream"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

const nihongo = "日本語"

type Example struct {
	Inputs []string
	Output string
}

var _ = Describe("UTF8Stream", func() {
	It("does not transmit utf8 codepoints that are split in twain", func() {
		buf := gbytes.NewBuffer()

		stream := NewWriter(buf)

		stream.Write([]byte(nihongo[:7]))
		Ω(buf.Contents()).Should(BeEmpty())

		stream.Write([]byte(nihongo[7:]))
		Ω(string(buf.Contents())).Should(Equal(nihongo))

		stream.Close()
		Ω(buf.Closed()).Should(BeTrue())
	})
})
