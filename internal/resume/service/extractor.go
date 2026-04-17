package service

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

type TextExtractor struct{}

func NewTextExtractor() *TextExtractor {
	return &TextExtractor{}
}

func (e *TextExtractor) Extract(fileType string, data []byte) (string, error) {
	switch fileType {
	case ".pdf":
		return e.extractFromPDF(data)
	case ".docx", ".doc":
		return e.extractFromDocx(data)
	case ".png", ".jpg", ".jpeg":
		return e.extractFromImage(data)
	default:
		return "", ErrFileTypeUnsupported
	}
}

func (e *TextExtractor) extractFromPDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("读取PDF失败: %w", err)
	}

	var buf strings.Builder
	totalPages := reader.NumPage()
	for i := 1; i <= totalPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		pageText := e.extractPDFPageText(page)
		if pageText != "" {
			buf.WriteString(pageText)
		}
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("PDF中未提取到文本内容，可能是扫描版PDF")
	}
	return result, nil
}

type pdfTextLine struct {
	y     float64
	items []pdf.Text
}

func (e *TextExtractor) extractPDFPageText(page pdf.Page) string {
	content := page.Content()
	if len(content.Text) == 0 {
		text, err := page.GetPlainText(nil)
		if err != nil {
			return ""
		}
		return text
	}

	var texts []pdf.Text
	for _, t := range content.Text {
		if strings.TrimSpace(t.S) != "" {
			texts = append(texts, t)
		}
	}
	if len(texts) == 0 {
		return ""
	}

	var lines []pdfTextLine
	for _, t := range texts {
		added := false
		for i := range lines {
			if math.Abs(t.Y-lines[i].y) < 3 {
				lines[i].items = append(lines[i].items, t)
				added = true
				break
			}
		}
		if !added {
			lines = append(lines, pdfTextLine{y: t.Y, items: []pdf.Text{t}})
		}
	}

	sort.Slice(lines, func(i, j int) bool {
		return lines[i].y > lines[j].y
	})

	for i := range lines {
		sort.Slice(lines[i].items, func(j, k int) bool {
			return lines[i].items[j].X < lines[i].items[k].X
		})
	}

	colXs := e.detectColumns(lines)

	if len(colXs) >= 2 {
		return e.formatByColumn(lines, colXs)
	}

	return e.formatByRow(lines)
}

func (e *TextExtractor) detectColumns(lines []pdfTextLine) []float64 {
	xCounts := make(map[int]int)
	for _, l := range lines {
		seen := make(map[int]bool)
		for _, t := range l.items {
			rx := int(math.Round(t.X/10) * 10)
			if !seen[rx] {
				xCounts[rx]++
				seen[rx] = true
			}
		}
	}

	minLines := len(lines) * 3 / 10
	if minLines < 2 {
		minLines = 2
	}

	var candidates []float64
	for rx, count := range xCounts {
		if count >= minLines {
			candidates = append(candidates, float64(rx))
		}
	}
	sort.Float64s(candidates)

	if len(candidates) < 2 {
		return nil
	}

	var groups []float64
	groupStart := candidates[0]
	groupEnd := candidates[0]

	for i := 1; i < len(candidates); i++ {
		if candidates[i]-groupEnd <= 40 {
			groupEnd = candidates[i]
		} else {
			groups = append(groups, (groupStart+groupEnd)/2)
			groupStart = candidates[i]
			groupEnd = candidates[i]
		}
	}
	groups = append(groups, (groupStart+groupEnd)/2)

	hasSignificantGap := false
	for i := 1; i < len(groups); i++ {
		if groups[i]-groups[i-1] > 50 {
			hasSignificantGap = true
			break
		}
	}

	if !hasSignificantGap {
		return nil
	}
	return groups
}

func (e *TextExtractor) formatByColumn(lines []pdfTextLine, colXs []float64) string {
	type columnLine struct {
		y     float64
		items []pdf.Text
	}
	type columnData struct {
		x     float64
		lines []columnLine
	}
	cols := make([]columnData, len(colXs))
	for i, x := range colXs {
		cols[i].x = x
	}

	for _, l := range lines {
		colItems := make([][]pdf.Text, len(colXs))
		for _, t := range l.items {
			best := 0
			bestDist := math.Abs(t.X - cols[0].x)
			for i := 1; i < len(cols); i++ {
				d := math.Abs(t.X - cols[i].x)
				if d < bestDist {
					bestDist = d
					best = i
				}
			}
			colItems[best] = append(colItems[best], t)
		}
		for i, items := range colItems {
			if len(items) > 0 {
				cols[i].lines = append(cols[i].lines, columnLine{y: l.y, items: items})
			}
		}
	}

	var buf strings.Builder
	for _, col := range cols {
		for _, l := range col.lines {
			lineText := e.mergeLineItems(l.items)
			buf.WriteString(lineText)
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

func (e *TextExtractor) formatByRow(lines []pdfTextLine) string {
	var buf strings.Builder
	for i, l := range lines {
		if i > 0 {
			buf.WriteString("\n")
		}
		lineText := e.mergeLineItems(l.items)
		buf.WriteString(lineText)
	}
	return buf.String()
}

func (e *TextExtractor) mergeLineItems(items []pdf.Text) string {
	if len(items) == 0 {
		return ""
	}

	var avgFontSize float64
	for _, t := range items {
		avgFontSize += t.FontSize
	}
	avgFontSize /= float64(len(items))

	if avgFontSize < 1 {
		avgFontSize = 12
	}

	spaceThreshold := avgFontSize * 0.4
	tabThreshold := avgFontSize * 2.5

	var buf strings.Builder
	for j, t := range items {
		if j > 0 {
			prev := items[j-1]
			gap := t.X - (prev.X + prev.W)

			if gap <= spaceThreshold {
				prevHasCJK := e.containsCJK(prev.S)
				currHasCJK := e.containsCJK(t.S)
				if prevHasCJK || currHasCJK {
					// CJK text: no space between adjacent characters
				} else {
					buf.WriteString("")
				}
			} else if gap <= tabThreshold {
				buf.WriteString(" ")
			} else {
				buf.WriteString("\t")
			}
		}
		buf.WriteString(t.S)
	}
	return buf.String()
}

func (e *TextExtractor) containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			return true
		}
	}
	return false
}

func (e *TextExtractor) extractFromDocx(data []byte) (string, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("读取DOCX失败: %w", err)
	}

	var buf strings.Builder
	for _, file := range zipReader.File {
		if file.Name == "word/document.xml" {
			text, err := e.parseDocxXML(file)
			if err != nil {
				return "", fmt.Errorf("解析DOCX内容失败: %w", err)
			}
			buf.WriteString(text)
			break
		}
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("DOCX中未提取到文本内容")
	}
	return result, nil
}

func (e *TextExtractor) parseDocxXML(file *zip.File) (string, error) {
	rc, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开document.xml失败: %w", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("读取document.xml失败: %w", err)
	}

	decoder := xml.NewDecoder(bytes.NewReader(content))
	var buf strings.Builder
	var inText bool
	var inTableCell bool
	var cellBuf strings.Builder
	var rowCells []string

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tc":
				inTableCell = true
				cellBuf.Reset()
			}
		case xml.CharData:
			if inText {
				if inTableCell {
					cellBuf.WriteString(string(t))
				} else {
					buf.WriteString(string(t))
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inTableCell {
					cellBuf.WriteString("\n")
				} else {
					buf.WriteString("\n")
				}
			case "t":
				inText = false
			case "tc":
				inTableCell = false
				cellContent := strings.TrimSpace(cellBuf.String())
				if cellContent != "" {
					rowCells = append(rowCells, cellContent)
				}
			case "tr":
				if len(rowCells) > 0 {
					buf.WriteString(strings.Join(rowCells, "\t"))
					buf.WriteString("\n")
				}
				rowCells = rowCells[:0]
			}
		}
	}

	return buf.String(), nil
}

func (e *TextExtractor) extractFromImage(data []byte) (string, error) {
	return "", fmt.Errorf("图片文件暂不支持文本提取，请将简历转换为PDF或DOCX格式后上传")
}
