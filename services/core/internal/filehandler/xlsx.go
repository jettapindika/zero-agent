package filehandler

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

type XlsxContent struct {
	Path     string
	OrigName string
	Sheets   []SheetContent
}

type SheetContent struct {
	Name string
	Rows [][]string
}

func ReadXlsx(path, origName string) (*XlsxContent, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open xlsx %s: %w", path, err)
	}
	defer f.Close()

	out := &XlsxContent{Path: path, OrigName: origName}
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name)
		if err != nil {
			rows = [][]string{{fmt.Sprintf("[error reading sheet: %v]", err)}}
		}
		out.Sheets = append(out.Sheets, SheetContent{Name: name, Rows: rows})
	}
	return out, nil
}

func (x *XlsxContent) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Excel: %s | %d sheet(s)]\n", x.OrigName, len(x.Sheets))
	for _, sheet := range x.Sheets {
		fmt.Fprintf(&b, "\nSheet: %s\n", sheet.Name)
		for _, row := range sheet.Rows {
			b.WriteString(strings.Join(row, ","))
			b.WriteByte('\n')
		}
	}
	return b.String()
}
