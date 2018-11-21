package eventstream_test

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/concourse/concourse/fly/eventstream"
)

var _ = Describe("Prefix Writer", func() {
	var buffer *bytes.Buffer
	var writer *eventstream.PrefixWriter

	BeforeEach(func() {
			buffer = &bytes.Buffer{}
			writer = eventstream.NewPrefixedWriter("time ", buffer)
		})

	It("should write prefix and two spaces at the beginning of each line", func() {
		writer.Write([]byte("first line\n"))
		writer.Write([]byte(" \n"))
		writer.Write([]byte("error log\nsome output\nmore string\n"))
		writer.Write([]byte("end of line"))

		Expect(buffer.String()).Should(Equal(`time first line
time  
time error log
time some output
time more string
time end of line`))
	})
})