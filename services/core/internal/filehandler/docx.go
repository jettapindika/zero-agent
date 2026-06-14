package filehandler

import (
	"fmt"
	"os"
	"strings"

	"github.com/fumiama/go-docx"
)

type DocxContent struct {
	Path     string
	OrigName string
	Text     string
}

func ReadDocx(path, origName string) (*DocxContent, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat docx %s: %w", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open docx %s: %w", path, err)
	}
	defer f.Close()

	doc, err := docx.Parse(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("parse docx %s: %w", path, err)
	}

	var b strings.Builder
	for _, item := range doc.Document.Body.Items {
		switch p := item.(type) {
		case *docx.Paragraph:
			for _, child := range p.Children {
				if run, ok := child.(*docx.Run); ok {
					for _, rc := range run.Children {
						if t, ok := rc.(*docx.Text); ok {
							b.WriteString(t.Text)
						}
					}
				}
			}
			b.WriteByte('\n')
		case *docx.Table:
			for _, row := range p.TableRows {
				cells := make([]string, 0, len(row.TableCells))
				for _, cell := range row.TableCells {
					var cb strings.Builder
					for _, item := range cell.Paragraphs {
						for _, child := range item.Children {
							if run, ok := child.(*docx.Run); ok {
								for _, rc := range run.Children {
									if t, ok := rc.(*docx.Text); ok {
										cb.WriteString(t.Text)
									}
								}
							}
						}
					}
					cells = append(cells, cb.String())
				}
				b.WriteString(strings.Join(cells, "\t"))
				b.WriteByte('\n')
			}
		}
	}
	return &DocxContent{Path: path, OrigName: origName, Text: strings.TrimRight(b.String(), "\n") + "\n"}, nil
}

func (d *DocxContent) String() string {
	return fmt.Sprintf("[Word document: %s]\n%s", d.OrigName, d.Text)
}
