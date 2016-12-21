package ui

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/fatih/color"
	colorable "github.com/mattn/go-colorable"
	"github.com/mattn/go-isatty"
)

type Table struct {
	Headers TableRow
	Data    Data
}

type Data []TableRow

type TableRow []TableCell

type TableCell struct {
	Contents string
	Color    *color.Color
}

func (d Data) Len() int          { return len(d) }
func (d Data) Swap(i int, j int) { d[i], d[j] = d[j], d[i] }

func (d Data) Less(i int, j int) bool {
	return d[i][0].Contents < d[j][0].Contents
}

func (table Table) Render(dst io.Writer) error {
	isTTY := false
	if file, ok := dst.(*os.File); ok && isatty.IsTerminal(file.Fd()) {
		isTTY = true
		if runtime.GOOS == "windows" {
			dst = colorable.NewColorable(file)
		}
	}

	columnWidths := map[int]int{}

	if isTTY {
		for i, column := range table.Headers {
			columnWidth := len(column.Contents)

			if columnWidth > columnWidths[i] {
				columnWidths[i] = columnWidth
			}
		}
	}

	for _, row := range table.Data {
		for i, column := range row {
			columnWidth := len(column.Contents)

			if columnWidth > columnWidths[i] {
				columnWidths[i] = columnWidth
			}
		}
	}

	if isTTY && table.Headers != nil {
		err := table.renderRow(dst, table.Headers, columnWidths, isTTY)
		if err != nil {
			return err
		}
	}

	for _, row := range table.Data {
		err := table.renderRow(dst, row, columnWidths, isTTY)
		if err != nil {
			return err
		}
	}

	return nil
}

func (table Table) renderRow(dst io.Writer, row TableRow, widths map[int]int, isTTY bool) error {
	for i, column := range row {
		if column.Color != nil {
			if isTTY {
				column.Color.EnableColor()
			} else {
				column.Color.DisableColor()
			}
		}

		contents := column.Contents
		if column.Color != nil {
			contents = column.Color.SprintFunc()(contents)
		}

		_, err := fmt.Fprintf(dst, "%s", contents)
		if err != nil {
			return err
		}

		paddingSize := widths[i] - len(column.Contents)
		_, err = fmt.Fprintf(dst, strings.Repeat(" ", paddingSize))
		if err != nil {
			return err
		}

		if i+1 < len(widths) {
			_, err := fmt.Fprintf(dst, "  ")
			if err != nil {
				return err
			}
		}
	}

	_, err := fmt.Fprintln(dst)
	if err != nil {
		return err
	}

	return nil
}
