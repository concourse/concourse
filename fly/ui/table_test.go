package ui_test

import (
	"io"
	"runtime"
	"sort"

	"github.com/concourse/fly/pty"
	. "github.com/concourse/fly/ui"
	"github.com/fatih/color"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Table", func() {
	var table Table

	BeforeEach(func() {
		table = Table{
			Headers: TableRow{
				{Contents: "column1", Color: color.New(color.Bold)},
				{Contents: "column2", Color: color.New(color.Bold)},
			},
			Data: []TableRow{
				{
					{Contents: "r1c1"},
					{Contents: "r1c2"},
				},
				{
					{Contents: "r2c1"},
					{Contents: "r2c2"},
				},
				{
					{Contents: "r3c1"},
					{Contents: "r3c2"},
				},
			},
		}
	})

	Describe("Sort", func() {
		Context("when rows are provided", func() {
			BeforeEach(func() {
				table = Table{
					Headers: TableRow{
						{Contents: "column1", Color: color.New(color.Bold)},
						{Contents: "column2", Color: color.New(color.Bold)},
					},
					Data: []TableRow{
						{
							{Contents: "zzz"},
							{Contents: "bbb"},
						},
						{
							{Contents: "aaa"},
							{Contents: "zzz"},
						},
						{
							{Contents: "xxx"},
							{Contents: "aaa"},
						},
					},
				}
			})

			It("sorts them on the given column index", func() {
				expectedOutput := "" +
					"aaa  zzz\n" +
					"xxx  aaa\n" +
					"zzz  bbb\n"

				buf := gbytes.NewBuffer()

				sort.Sort(table.Data)

				err := table.Render(buf, false)
				Expect(err).ToNot(HaveOccurred())

				Expect(string(buf.Contents())).To(Equal(expectedOutput))
			})
		})
	})

	Describe("Render", func() {
		Context("when the render method is called without a TTY", func() {
			It("prints the data with no headers", func() {
				expectedOutput := "" +
					"r1c1  r1c2\n" +
					"r2c1  r2c2\n" +
					"r3c1  r3c2\n"

				buf := gbytes.NewBuffer()

				err := table.Render(buf, false)
				Expect(err).ToNot(HaveOccurred())

				Expect(string(buf.Contents())).To(Equal(expectedOutput))
			})
		})

		Context("when the render method is called without a TTY but with print headers flag", func() {
			It("prints the headers and the data without color", func() {
				buf := gbytes.NewBuffer()

				err := table.Render(buf, true)
				Expect(err).ToNot(HaveOccurred())

				expectedOutput := "" +
					"column1  column2\n" +
					"r1c1     r1c2   \n" +
					"r2c1     r2c2   \n" +
					"r3c1     r3c2   \n"

				Expect(string(buf.Contents())).To(Equal(expectedOutput))
			})
		})

		Context("when the render method is called in a TTY", func() {
			It("prints the headers and the data in color", func() {
				if runtime.GOOS == "windows" {
					Skip("these escape codes, and the pty stuff, don't apply to Windows")
				}

				pty, err := pty.Open()
				Expect(err).NotTo(HaveOccurred())

				defer pty.Close()

				buf := gbytes.NewBuffer()

				go io.Copy(buf, pty.PTYR)

				err = table.Render(pty.TTYW, false)
				Expect(err).ToNot(HaveOccurred())

				expectedOutput := "" +
					"\x1b[1mcolumn1\x1b[0m  \x1b[1mcolumn2\x1b[0m\r\n" +
					"r1c1     r1c2   \r\n" +
					"r2c1     r2c2   \r\n" +
					"r3c1     r3c2   \r\n"

				Eventually(buf.Contents).Should(Equal([]byte(expectedOutput)))
			})
		})
	})
})
