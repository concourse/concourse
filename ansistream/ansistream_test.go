package ansistream_test

import (
	"fmt"

	. "github.com/winston-ci/winston/ansistream"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

const nihongo = "日本語"

type Example struct {
	Inputs []string
	Output string
}

var _ = Describe("AnsiStream", func() {
	examples := []Example{
		{[]string{"\x1b[1mfoo"}, `<span class="ansi-bold">foo</span>`},

		{[]string{"\x1b[30mfoo"}, `<span class="ansi-black">foo</span>`},
		{[]string{"\x1b[31mfoo"}, `<span class="ansi-red">foo</span>`},
		{[]string{"\x1b[32mfoo"}, `<span class="ansi-green">foo</span>`},
		{[]string{"\x1b[33mfoo"}, `<span class="ansi-yellow">foo</span>`},
		{[]string{"\x1b[34mfoo"}, `<span class="ansi-blue">foo</span>`},
		{[]string{"\x1b[35mfoo"}, `<span class="ansi-magenta">foo</span>`},
		{[]string{"\x1b[36mfoo"}, `<span class="ansi-cyan">foo</span>`},
		{[]string{"\x1b[37mfoo"}, `<span class="ansi-white">foo</span>`},

		{[]string{"fizz\x1b[30mbuzz"}, `fizz<span class="ansi-black">buzz</span>`},
		{[]string{"fizz\x1b[31mbuzz"}, `fizz<span class="ansi-red">buzz</span>`},
		{[]string{"fizz\x1b[32mbuzz"}, `fizz<span class="ansi-green">buzz</span>`},
		{[]string{"fizz\x1b[33mbuzz"}, `fizz<span class="ansi-yellow">buzz</span>`},
		{[]string{"fizz\x1b[34mbuzz"}, `fizz<span class="ansi-blue">buzz</span>`},
		{[]string{"fizz\x1b[35mbuzz"}, `fizz<span class="ansi-magenta">buzz</span>`},
		{[]string{"fizz\x1b[36mbuzz"}, `fizz<span class="ansi-cyan">buzz</span>`},
		{[]string{"fizz\x1b[37mbuzz"}, `fizz<span class="ansi-white">buzz</span>`},

		{[]string{"\x1b[1;30mfoo"}, `<span class="ansi-bold ansi-black">foo</span>`},
		{[]string{"\x1b[1;31mfoo"}, `<span class="ansi-bold ansi-red">foo</span>`},
		{[]string{"\x1b[1;32mfoo"}, `<span class="ansi-bold ansi-green">foo</span>`},
		{[]string{"\x1b[1;33mfoo"}, `<span class="ansi-bold ansi-yellow">foo</span>`},
		{[]string{"\x1b[1;34mfoo"}, `<span class="ansi-bold ansi-blue">foo</span>`},
		{[]string{"\x1b[1;35mfoo"}, `<span class="ansi-bold ansi-magenta">foo</span>`},
		{[]string{"\x1b[1;36mfoo"}, `<span class="ansi-bold ansi-cyan">foo</span>`},
		{[]string{"\x1b[1;37mfoo"}, `<span class="ansi-bold ansi-white">foo</span>`},

		{[]string{"\x1b[90mfoo"}, `<span class="ansi-bright ansi-black">foo</span>`},
		{[]string{"\x1b[91mfoo"}, `<span class="ansi-bright ansi-red">foo</span>`},
		{[]string{"\x1b[92mfoo"}, `<span class="ansi-bright ansi-green">foo</span>`},
		{[]string{"\x1b[93mfoo"}, `<span class="ansi-bright ansi-yellow">foo</span>`},
		{[]string{"\x1b[94mfoo"}, `<span class="ansi-bright ansi-blue">foo</span>`},
		{[]string{"\x1b[95mfoo"}, `<span class="ansi-bright ansi-magenta">foo</span>`},
		{[]string{"\x1b[96mfoo"}, `<span class="ansi-bright ansi-cyan">foo</span>`},
		{[]string{"\x1b[97mfoo"}, `<span class="ansi-bright ansi-white">foo</span>`},

		{[]string{"\x1b[1;90mfoo"}, `<span class="ansi-bold ansi-bright ansi-black">foo</span>`},
		{[]string{"\x1b[1;91mfoo"}, `<span class="ansi-bold ansi-bright ansi-red">foo</span>`},
		{[]string{"\x1b[1;92mfoo"}, `<span class="ansi-bold ansi-bright ansi-green">foo</span>`},
		{[]string{"\x1b[1;93mfoo"}, `<span class="ansi-bold ansi-bright ansi-yellow">foo</span>`},
		{[]string{"\x1b[1;94mfoo"}, `<span class="ansi-bold ansi-bright ansi-blue">foo</span>`},
		{[]string{"\x1b[1;95mfoo"}, `<span class="ansi-bold ansi-bright ansi-magenta">foo</span>`},
		{[]string{"\x1b[1;96mfoo"}, `<span class="ansi-bold ansi-bright ansi-cyan">foo</span>`},
		{[]string{"\x1b[1;97mfoo"}, `<span class="ansi-bold ansi-bright ansi-white">foo</span>`},

		// terminated
		{[]string{"\x1b[31mfoo\x1b[0m"}, `<span class="ansi-red">foo</span>`},
		{[]string{"\x1b[31mfoo\x1b[0mbar"}, `<span class="ansi-red">foo</span>bar`},

		// cut in the middle
		{[]string{"\x1b[1;9", "0mfoo"}, `<span class="ansi-bold ansi-bright ansi-black">foo</span>`},
		{[]string{"\x1b", "[1;9", "0mfoo"}, `<span class="ansi-bold ansi-bright ansi-black">foo</span>`},
		{[]string{"\x1b[1m", "foo"}, `<span class="ansi-bold">foo</span>`},
		{[]string{"\x1b[1m\x1b[0m", "foo"}, `foo`},
		{[]string{"\x1b[1mfoo\x1b[0m", "bar"}, `<span class="ansi-bold">foo</span>bar`},
		{[]string{"\x1b[1m" + nihongo[:1], nihongo[1:]}, `<span class="ansi-bold">` + nihongo + `</span>`},
		{[]string{"\x1b[1mfoo", "bar"}, `<span class="ansi-bold">foo</span><span class="ansi-bold">bar</span>`},

		// codeA -> normal -> empty -> codeA (unify; optimization)
		{
			[]string{"\x1b[1;91mfoo\x1b[0m\x1b[1;91mbar"},
			`<span class="ansi-bold ansi-bright ansi-red">foobar</span>`,
		},

		// codeA -> normal -> text -> codeA (ensure we reinstate same style)
		{
			[]string{"\x1b[1;91mfoo\x1b[0mhello\x1b[1;91mbar"},
			`<span class="ansi-bold ansi-bright ansi-red">foo</span>hello<span class="ansi-bold ansi-bright ansi-red">bar</span>`,
		},

		// check that we remember styles across boundaries
		{[]string{"\x1b[1m", "\x1b[0m\x1b[1mfoo"}, `<span class="ansi-bold">foo</span>`},
		{[]string{"\x1b[1m", "\x1b[1mfoo"}, `<span class="ansi-bold">foo</span>`},

		// non-ansi escape sequence
		{[]string{"\x1bsomebogusdata"}, "\x1bsomebogusdata"},

		// escape html characters
		{[]string{"\x1b[1m<foo> & bar\x1b[0m"}, `<span class="ansi-bold">&lt;foo&gt; &amp; bar</span>`},
	}

	for _, example := range examples {
		inputs := example.Inputs
		output := example.Output

		Context(fmt.Sprintf("with inputs %q", inputs), func() {
			It(fmt.Sprintf("renders %s", output), func() {
				buf := gbytes.NewBuffer()

				stream := NewWriter(buf)

				for _, chunk := range inputs {
					stream.Write([]byte(chunk))
				}

				stream.Close()

				Ω(buf.Contents()).Should(Equal([]byte(output)))
				Ω(buf.Closed()).Should(BeTrue())
			})
		})
	}

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
