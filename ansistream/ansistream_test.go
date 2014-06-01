package ansistream_test

import (
	"fmt"

	. "github.com/winston-ci/winston/ansistream"
	"github.com/winston-ci/winston/utf8stream"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

type Example struct {
	Inputs []string
	Output string
}

var _ = Describe("AnsiStream", func() {
	examples := []Example{
		{[]string{"\x1b[1mfoo"}, `<span class="ansi-bold">foo</span>`},

		{[]string{"\x1b[30mfoo"}, `<span class="ansi-black-fg">foo</span>`},
		{[]string{"\x1b[31mfoo"}, `<span class="ansi-red-fg">foo</span>`},
		{[]string{"\x1b[32mfoo"}, `<span class="ansi-green-fg">foo</span>`},
		{[]string{"\x1b[33mfoo"}, `<span class="ansi-yellow-fg">foo</span>`},
		{[]string{"\x1b[34mfoo"}, `<span class="ansi-blue-fg">foo</span>`},
		{[]string{"\x1b[35mfoo"}, `<span class="ansi-magenta-fg">foo</span>`},
		{[]string{"\x1b[36mfoo"}, `<span class="ansi-cyan-fg">foo</span>`},
		{[]string{"\x1b[37mfoo"}, `<span class="ansi-white-fg">foo</span>`},

		{[]string{"\x1b[40mfoo"}, `<span class="ansi-black-bg">foo</span>`},
		{[]string{"\x1b[41mfoo"}, `<span class="ansi-red-bg">foo</span>`},
		{[]string{"\x1b[42mfoo"}, `<span class="ansi-green-bg">foo</span>`},
		{[]string{"\x1b[43mfoo"}, `<span class="ansi-yellow-bg">foo</span>`},
		{[]string{"\x1b[44mfoo"}, `<span class="ansi-blue-bg">foo</span>`},
		{[]string{"\x1b[45mfoo"}, `<span class="ansi-magenta-bg">foo</span>`},
		{[]string{"\x1b[46mfoo"}, `<span class="ansi-cyan-bg">foo</span>`},
		{[]string{"\x1b[47mfoo"}, `<span class="ansi-white-bg">foo</span>`},

		{[]string{"fizz\x1b[30mbuzz"}, `fizz<span class="ansi-black-fg">buzz</span>`},
		{[]string{"fizz\x1b[31mbuzz"}, `fizz<span class="ansi-red-fg">buzz</span>`},
		{[]string{"fizz\x1b[32mbuzz"}, `fizz<span class="ansi-green-fg">buzz</span>`},
		{[]string{"fizz\x1b[33mbuzz"}, `fizz<span class="ansi-yellow-fg">buzz</span>`},
		{[]string{"fizz\x1b[34mbuzz"}, `fizz<span class="ansi-blue-fg">buzz</span>`},
		{[]string{"fizz\x1b[35mbuzz"}, `fizz<span class="ansi-magenta-fg">buzz</span>`},
		{[]string{"fizz\x1b[36mbuzz"}, `fizz<span class="ansi-cyan-fg">buzz</span>`},
		{[]string{"fizz\x1b[37mbuzz"}, `fizz<span class="ansi-white-fg">buzz</span>`},

		{[]string{"\x1b[1;30mfoo"}, `<span class="ansi-bold ansi-black-fg">foo</span>`},
		{[]string{"\x1b[1;31mfoo"}, `<span class="ansi-bold ansi-red-fg">foo</span>`},
		{[]string{"\x1b[1;32mfoo"}, `<span class="ansi-bold ansi-green-fg">foo</span>`},
		{[]string{"\x1b[1;33mfoo"}, `<span class="ansi-bold ansi-yellow-fg">foo</span>`},
		{[]string{"\x1b[1;34mfoo"}, `<span class="ansi-bold ansi-blue-fg">foo</span>`},
		{[]string{"\x1b[1;35mfoo"}, `<span class="ansi-bold ansi-magenta-fg">foo</span>`},
		{[]string{"\x1b[1;36mfoo"}, `<span class="ansi-bold ansi-cyan-fg">foo</span>`},
		{[]string{"\x1b[1;37mfoo"}, `<span class="ansi-bold ansi-white-fg">foo</span>`},

		{[]string{"\x1b[90mfoo"}, `<span class="ansi-bright-black-fg">foo</span>`},
		{[]string{"\x1b[91mfoo"}, `<span class="ansi-bright-red-fg">foo</span>`},
		{[]string{"\x1b[92mfoo"}, `<span class="ansi-bright-green-fg">foo</span>`},
		{[]string{"\x1b[93mfoo"}, `<span class="ansi-bright-yellow-fg">foo</span>`},
		{[]string{"\x1b[94mfoo"}, `<span class="ansi-bright-blue-fg">foo</span>`},
		{[]string{"\x1b[95mfoo"}, `<span class="ansi-bright-magenta-fg">foo</span>`},
		{[]string{"\x1b[96mfoo"}, `<span class="ansi-bright-cyan-fg">foo</span>`},
		{[]string{"\x1b[97mfoo"}, `<span class="ansi-bright-white-fg">foo</span>`},

		{[]string{"\x1b[100mfoo"}, `<span class="ansi-bright-black-bg">foo</span>`},
		{[]string{"\x1b[101mfoo"}, `<span class="ansi-bright-red-bg">foo</span>`},
		{[]string{"\x1b[102mfoo"}, `<span class="ansi-bright-green-bg">foo</span>`},
		{[]string{"\x1b[103mfoo"}, `<span class="ansi-bright-yellow-bg">foo</span>`},
		{[]string{"\x1b[104mfoo"}, `<span class="ansi-bright-blue-bg">foo</span>`},
		{[]string{"\x1b[105mfoo"}, `<span class="ansi-bright-magenta-bg">foo</span>`},
		{[]string{"\x1b[106mfoo"}, `<span class="ansi-bright-cyan-bg">foo</span>`},
		{[]string{"\x1b[107mfoo"}, `<span class="ansi-bright-white-bg">foo</span>`},

		{[]string{"\x1b[1;90mfoo"}, `<span class="ansi-bold ansi-bright-black-fg">foo</span>`},
		{[]string{"\x1b[1;91mfoo"}, `<span class="ansi-bold ansi-bright-red-fg">foo</span>`},
		{[]string{"\x1b[1;92mfoo"}, `<span class="ansi-bold ansi-bright-green-fg">foo</span>`},
		{[]string{"\x1b[1;93mfoo"}, `<span class="ansi-bold ansi-bright-yellow-fg">foo</span>`},
		{[]string{"\x1b[1;94mfoo"}, `<span class="ansi-bold ansi-bright-blue-fg">foo</span>`},
		{[]string{"\x1b[1;95mfoo"}, `<span class="ansi-bold ansi-bright-magenta-fg">foo</span>`},
		{[]string{"\x1b[1;96mfoo"}, `<span class="ansi-bold ansi-bright-cyan-fg">foo</span>`},
		{[]string{"\x1b[1;97mfoo"}, `<span class="ansi-bold ansi-bright-white-fg">foo</span>`},

		// terminated
		{[]string{"\x1b[31mfoo\x1b[0m"}, `<span class="ansi-red-fg">foo</span>`},
		{[]string{"\x1b[31mfoo\x1b[0mbar"}, `<span class="ansi-red-fg">foo</span>bar`},

		// cut in the middle
		{[]string{"\x1b[1;9", "0mfoo"}, `<span class="ansi-bold ansi-bright-black-fg">foo</span>`},
		{[]string{"\x1b", "[1;9", "0mfoo"}, `<span class="ansi-bold ansi-bright-black-fg">foo</span>`},
		{[]string{"\x1b[1m", "foo"}, `<span class="ansi-bold">foo</span>`},
		{[]string{"\x1b[1m\x1b[0m", "foo"}, `foo`},
		{[]string{"\x1b[1mfoo\x1b[0m", "bar"}, `<span class="ansi-bold">foo</span>bar`},
		{[]string{"\x1b[1mfoo", "bar"}, `<span class="ansi-bold">foo</span><span class="ansi-bold">bar</span>`},

		// codeA -> normal -> empty -> codeA (unify; optimization)
		{
			[]string{"\x1b[1;91mfoo\x1b[0m\x1b[1;91mbar"},
			`<span class="ansi-bold ansi-bright-red-fg">foobar</span>`,
		},

		// codeA -> normal -> text -> codeA (ensure we reinstate same style)
		{
			[]string{"\x1b[1;91mfoo\x1b[0mhello\x1b[1;91mbar"},
			`<span class="ansi-bold ansi-bright-red-fg">foo</span>hello<span class="ansi-bold ansi-bright-red-fg">bar</span>`,
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

				stream := utf8stream.NewWriter(NewWriter(buf))

				for _, chunk := range inputs {
					stream.Write([]byte(chunk))
				}

				stream.Close()

				Ω(buf.Contents()).Should(Equal([]byte(output)))
				Ω(buf.Closed()).Should(BeTrue())
			})
		})
	}
})
