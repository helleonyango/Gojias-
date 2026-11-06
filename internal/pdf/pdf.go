// Package pdf writes very small single-page text PDFs without any dependency.
//
// It is deliberately limited: Helvetica, absolute-positioned lines, A4. That is
// all the order summary needs, and it keeps the binary dependency-free.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// Line is one line of text in the document.
type Line struct {
	Text  string
	Size  float64 // point size; defaults to 11
	Bold  bool
	Space float64 // extra vertical space before the line
}

// Build renders the lines into a one-page A4 PDF.
func Build(lines []Line) []byte {
	const (
		pageWidth  = 595
		pageHeight = 842
		margin     = 56
	)

	var content strings.Builder
	y := float64(pageHeight - margin)
	for _, l := range lines {
		size := l.Size
		if size == 0 {
			size = 11
		}
		y -= l.Space
		if y < margin {
			break // single page only; callers keep summaries short
		}
		font := "/F1"
		if l.Bold {
			font = "/F2"
		}
		fmt.Fprintf(&content, "BT %s %.1f Tf %.1f %.1f Td (%s) Tj ET\n",
			font, size, float64(margin), y, escape(l.Text))
		y -= size * 1.55
	}

	stream := content.String()

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	var offsets []int
	addObject := func(body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}

	addObject("<< /Type /Catalog /Pages 2 0 R >>")
	addObject("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	addObject(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] "+
		"/Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>", pageWidth, pageHeight))
	addObject("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	addObject("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	addObject(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream))

	xrefPos := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets)+1, xrefPos)

	return buf.Bytes()
}

// escape makes text safe for a PDF string literal. Characters outside WinAnsi
// (typographic dashes, quotes, emoji) are replaced rather than emitted raw.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '(', ')':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if r > 255 {
				b.WriteByte('?')
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
