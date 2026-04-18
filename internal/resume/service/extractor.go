package service

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// TextExtractor 简历文本提取器，支持 PDF、DOCX、图片三种格式
type TextExtractor struct{}

// NewTextExtractor 创建 TextExtractor 实例
func NewTextExtractor() *TextExtractor {
	return &TextExtractor{}
}

// Extract 根据文件类型调用相应的提取方法
// 支持 .pdf、.docx/.doc、.png/.jpg/.jpeg 格式
func (e *TextExtractor) Extract(fileType string, data []byte) (string, error) {
	switch fileType {
	case ".pdf":
		return e.extractFromPDF(data)
	case ".docx", ".doc":
		return e.extractFromDOCX(data)
	case ".png", ".jpg", ".jpeg":
		return e.extractFromImage(data)
	default:
		return "", ErrFileTypeUnsupported
	}
}

// extractFromPDF 从 PDF 文件中提取文本
// 逐页读取，合并后进行段落标题格式化处理
func (e *TextExtractor) extractFromPDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("读取PDF失败: %w", err)
	}

	var buf strings.Builder
	totalPages := reader.NumPage()

	// 逐页提取文本内容
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

	// 对提取结果进行后处理：将段落标题（如"教育经历"）独占一行
	result = e.formatSectionHeaders(result)
	return result, nil
}

// pdfTextLine 表示 PDF 中同一行的文本片段集合
type pdfTextLine struct {
	y     float64    // 该行文本的 Y 坐标（用于判断同行）
	items []pdf.Text // 该行包含的所有文本片段
}

// extractPDFPageText 从单页 PDF 中提取文本
// 返回该页的完整文本内容
func (e *TextExtractor) extractPDFPageText(page pdf.Page) string {
	content := page.Content()

	// 如果页面没有 Content.Text（通常是扫描页），降级使用 GetPlainText
	if len(content.Text) == 0 {
		text, err := page.GetPlainText(nil)
		if err != nil {
			return ""
		}
		return text
	}

	// 过滤空文本片段
	var texts []pdf.Text
	for _, t := range content.Text {
		if strings.TrimSpace(t.S) != "" {
			texts = append(texts, t)
		}
	}
	if len(texts) == 0 {
		return ""
	}

	// 将文本片段按 Y 坐标分组到不同行
	// Y 坐标差值小于 3 的片段视为同一行
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

	// 按 Y 坐标从大到小排序（PDF Y 轴向下，大的在上，即从页顶到底）
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].y > lines[j].y
	})

	// 每行内按 X 坐标从小到大排序（从左到右）
	for i := range lines {
		sort.Slice(lines[i].items, func(j, k int) bool {
			return lines[i].items[j].X < lines[i].items[k].X
		})
	}

	// 检测是否为多列布局（通过统计各 X 坐标出现频率判断列间距）
	colXs := e.detectColumns(lines)

	if len(colXs) >= 2 {
		// 多列布局：按列分别输出
		return e.formatByColumn(lines, colXs)
	}
	// 单列布局：逐行输出
	return e.formatByRow(lines)
}

// detectColumns 检测 PDF 页面是否为多列布局
// 通过统计各 X 坐标的出现次数，找出形成列间距的 X 位置
// 返回值：多列布局时返回各列中心 X 坐标列表；单列时返回 nil
func (e *TextExtractor) detectColumns(lines []pdfTextLine) []float64 {
	// 统计各 X 坐标（按 10pt 取整）的出现次数
	xCounts := make(map[int]int)
	for _, l := range lines {
		seen := make(map[int]bool)
		for _, t := range l.items {
			rx := int(math.Round(t.X/10) * 10) // 取最近的 10 的倍数
			if !seen[rx] {
				xCounts[rx]++
				seen[rx] = true
			}
		}
	}

	// 至少在 30% 的行中出现的 X 坐标才可能是列位置
	minLines := len(lines) * 3 / 10
	if minLines < 2 {
		minLines = 2
	}

	// 收集满足阈值的 X 坐标候选
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

	// 将相邻（间距 ≤40）的候选合并为列组，取组中心为列 X 坐标
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

	// 列之间必须有足够大的间距（>50）才算多列布局
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

// formatByColumn 按列输出多列布局的文本
// 每列各自从上到下输出，列与列之间用换行分隔
func (e *TextExtractor) formatByColumn(lines []pdfTextLine, colXs []float64) string {
	type columnLine struct {
		y     float64
		items []pdf.Text
	}
	type columnData struct {
		x     float64
		lines []columnLine
	}

	// 初始化各列
	cols := make([]columnData, len(colXs))
	for i, x := range colXs {
		cols[i].x = x
	}

	// 将每行的文本片段分配到最近的列
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
		// 将非空的列片段加入对应列
		for i, items := range colItems {
			if len(items) > 0 {
				cols[i].lines = append(cols[i].lines, columnLine{y: l.y, items: items})
			}
		}
	}

	var buf strings.Builder
	// 每列内按行输出，列与列之间通过空行分隔
	for _, col := range cols {
		for _, l := range col.lines {
			// 先按字号拆分同一行中的段落标题（如"教育经历 东莞理工"拆为两行）
			groups := e.splitLineByFontSize(l.items)
			for gi, g := range groups {
				if gi > 0 {
					buf.WriteString("\n")
				}
				lineText := e.mergeLineItems(g)
				buf.WriteString(lineText)
			}
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

// formatByRow 按行输出单列布局的文本
func (e *TextExtractor) formatByRow(lines []pdfTextLine) string {
	var buf strings.Builder
	for i, l := range lines {
		if i > 0 {
			buf.WriteString("\n")
		}
		// 先按字号拆分同一行中的段落标题
		groups := e.splitLineByFontSize(l.items)
		for gi, g := range groups {
			if gi > 0 {
				buf.WriteString("\n")
			}
			lineText := e.mergeLineItems(g)
			buf.WriteString(lineText)
		}
	}
	return buf.String()
}

// splitLineByFontSize 将同行文本按字号大小拆分为多个组
// 用于处理 PDF 中段落标题与正文位于同一行但字号不同的情况
// 例如："教育经历 东莞理工学院" 中"教育经历"字号更大，会被拆为独立一行
// 返回：每组内的文本字号相近，不同组之间字号差异明显（≥15%）
func (e *TextExtractor) splitLineByFontSize(items []pdf.Text) [][]pdf.Text {
	if len(items) <= 1 {
		return [][]pdf.Text{items}
	}

	// 找出行内最小和最大字号
	var minFS, maxFS float64
	minFS = items[0].FontSize
	maxFS = items[0].FontSize
	for _, t := range items[1:] {
		if t.FontSize < minFS {
			minFS = t.FontSize
		}
		if t.FontSize > maxFS {
			maxFS = t.FontSize
		}
	}

	// 字号差异小于 15% 视为同一层级，不拆分
	if maxFS <= 0 || minFS <= 0 || maxFS/minFS < 1.15 {
		return [][]pdf.Text{items}
	}

	// 按字号变化点分组
	var groups [][]pdf.Text
	var current []pdf.Text
	currentSize := items[0].FontSize

	for _, t := range items {
		if len(current) > 0 && math.Abs(t.FontSize-currentSize) > currentSize*0.1 {
			// 字号变化超过 10% 时开启新组
			groups = append(groups, current)
			current = nil
			currentSize = t.FontSize
		}
		current = append(current, t)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}

	if len(groups) <= 1 {
		return [][]pdf.Text{items}
	}

	return groups
}

// mergeLineItems 将同行文本片段合并为一个字符串
// 根据相邻片段的 X 坐标间距和字符类型（CJK/非CJK）决定是否插入空格或制表符
// - CJK 字符之间：紧邻（间距小）无分隔，中等间距加空格，大间距加制表符
// - 非 CJK 字符之间：用于保留英文单词间的自然间距
// - 混合场景：谨慎处理，避免引入多余空格
func (e *TextExtractor) mergeLineItems(items []pdf.Text) string {
	if len(items) == 0 {
		return ""
	}

	// 计算该行平均字号，用于估算缺失宽度（W=0 时的处理）
	var avgFontSize float64
	for _, t := range items {
		avgFontSize += t.FontSize
	}
	avgFontSize /= float64(len(items))
	if avgFontSize < 1 {
		avgFontSize = 12
	}

	var buf strings.Builder
	for j, t := range items {
		if j > 0 {
			prev := items[j-1]
			prevHasCJK := e.containsCJK(prev.S)
			currHasCJK := e.containsCJK(t.S)

			var gap float64
			if prev.W > 0 {
				// 上一片段有有效宽度，直接用 X 坐标计算真实间距
				gap = t.X - (prev.X + prev.W)
			} else {
				// 上一片段宽度缺失（W=0），基于字号估算字符宽度和间距
				xDist := t.X - prev.X
				if prevHasCJK && currHasCJK {
					// 双方都是 CJK：CJK 字符宽度约等于字号
					if xDist < avgFontSize*0.5 {
						gap = 0
					} else {
						gap = xDist - avgFontSize
					}
				} else if !prevHasCJK && !currHasCJK {
					// 双方都是非 CJK：拉丁字符宽度约等于半字号
					if xDist < avgFontSize*0.6 {
						gap = 0
					} else {
						gap = xDist - avgFontSize*0.5
					}
				} else {
					// 混合场景：字符宽度介于两者之间
					if xDist < avgFontSize*0.55 {
						gap = 0
					} else {
						gap = xDist - avgFontSize*0.7
					}
				}
			}

			// 根据 gap 大小决定分隔符
			if prevHasCJK && currHasCJK {
				// CJK 紧邻（gap < 0.5字号）无分隔；中等间距加空格；大间距加制表符
				if gap < avgFontSize*0.5 {
					// 紧邻，不加分隔
				} else if gap < avgFontSize*2 {
					buf.WriteString(" ")
				} else {
					buf.WriteString("\t")
				}
			} else if prevHasCJK || currHasCJK {
				// 混合场景更保守，避免多余空格
				if gap < avgFontSize*0.3 {
				} else if gap < avgFontSize*1.5 {
					buf.WriteString(" ")
				} else {
					buf.WriteString("\t")
				}
			} else {
				// 非 CJK 字符间：正常单词间距保留
				if gap < avgFontSize*0.5 {
				} else if gap < avgFontSize*1.5 {
					buf.WriteString(" ")
				} else {
					buf.WriteString("\t")
				}
			}
		}
		buf.WriteString(t.S)
	}
	return buf.String()
}

// containsCJK 检测字符串中是否包含 CJK（中日韩）字符
func (e *TextExtractor) containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) ||
			unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			return true
		}
	}
	return false
}

// extractFromDOCX 从 DOCX 文件中提取文本
// DOCX 本质上是 ZIP 压缩包，文本内容存储在 word/document.xml 中
func (e *TextExtractor) extractFromDOCX(data []byte) (string, error) {
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

// parseDocxXML 解析 DOCX 的 document.xml，提取纯文本
// 处理 XML 中的 <t>（文本）、<p>（段落）、<tc>（表格单元格）、<tr>（表格行）等标签
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
	var inText bool             // 是否在 <t> 标签内
	var inTableCell bool        // 是否在表格单元格内
	var cellBuf strings.Builder // 单元格内容缓冲
	var rowCells []string       // 当前行的单元格列表

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
				// 进入表格单元格，重置单元格缓冲
				inTableCell = true
				cellBuf.Reset()
			}
		case xml.CharData:
			// 文本内容：如果是单元格内则写入缓冲，否则直接写入主缓冲
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
				// 段落结束：换行（单元格内也换行）
				if inTableCell {
					cellBuf.WriteString("\n")
				} else {
					buf.WriteString("\n")
				}
			case "t":
				inText = false
			case "tc":
				// 单元格结束：将缓冲内容加入行单元格列表
				inTableCell = false
				cellContent := strings.TrimSpace(cellBuf.String())
				if cellContent != "" {
					rowCells = append(rowCells, cellContent)
				}
			case "tr":
				// 表格行结束：将该行所有单元格用制表符连接并写入主缓冲
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

// extractFromImage 图片文件暂不支持文本提取
func (e *TextExtractor) extractFromImage(data []byte) (string, error) {
	return "", fmt.Errorf("图片文件暂不支持文本提取，请将简历转换为PDF或DOCX格式后上传")
}

// sectionHeaderRe 预编译的段落标题正则表达式
// 匹配常见简历段落标题：教育经历、专业技能、项目经验、实习经历 等
var sectionHeaderRe *regexp.Regexp

func init() {
	headers := []string{
		"教育经历", "教育背景",
		"专业技能", "技能特长",
		"项目经历", "项目经验",
		"实习经历", "工作经历", "工作经验",
		"荣誉奖项", "获奖情况",
		"自我评价", "个人简介",
		"校园经历", "社会实践",
		"求职意向",
	}
	pattern := "(" + strings.Join(headers, "|") + ")"
	sectionHeaderRe = regexp.MustCompile(pattern)
}

// formatSectionHeaders 将段落标题与后续内容拆分为不同行
// 处理两种情况：
// 1. 标题前有内容且同行："个人介绍 张三" → "个人介绍\n张三"
// 2. 标题后紧跟内容："教育经历 东莞理工" → "教育经历\n东莞理工"
// 这是一个兜底处理，主要的标题检测在 splitLineByFontSize 中基于字号完成
func (e *TextExtractor) formatSectionHeaders(text string) string {
	// 标题前有内容同行的情况：非换行 + 空格/制表符 + 标题
	reBefore := regexp.MustCompile("([^\n])[ \t]+" + sectionHeaderRe.String())
	text = reBefore.ReplaceAllString(text, "$1\n$2")

	// 标题后紧跟内容的情况：标题 + 空格/制表符 + 非换行
	reAfter := regexp.MustCompile(sectionHeaderRe.String() + "[ \t]+([^\n])")
	text = reAfter.ReplaceAllString(text, "$1\n$2")

	return text
}
