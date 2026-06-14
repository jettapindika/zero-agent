package filehandler

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

type PDFContent struct {
	Path      string
	OrigName  string
	PageCount int
	Pages     []PDFPage
}

type PDFPage struct {
	Number int
	Text   string
}

func ReadPDF(path, origName string) (*PDFContent, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf %s: %w", path, err)
	}
	defer f.Close()

	total := r.NumPage()
	out := &PDFContent{
		Path:      path,
		OrigName:  origName,
		PageCount: total,
		Pages:     make([]PDFPage, 0, total),
	}
	for i := 1; i <= total; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			text = fmt.Sprintf("[page %d: could not extract text: %v]", i, err)
		}
		out.Pages = append(out.Pages, PDFPage{Number: i, Text: text})
	}
	return out, nil
}

func (p *PDFContent) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[PDF: %s | %d pages]\n", p.OrigName, p.PageCount)
	for _, page := range p.Pages {
		fmt.Fprintf(&b, "\n--- Page %d ---\n%s", page.Number, strings.TrimSpace(page.Text))
	}
	return b.String()
}
