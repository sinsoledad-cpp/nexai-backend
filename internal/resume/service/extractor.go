package service

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

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
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		buf.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			buf.WriteString("\n")
		}
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("PDF中未提取到文本内容，可能是扫描版PDF")
	}
	return result, nil
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
			if t.Name.Local == "t" {
				inText = true
			}
		case xml.CharData:
			if inText {
				buf.WriteString(string(t))
			}
		case xml.EndElement:
			if t.Name.Local == "p" {
				buf.WriteString("\n")
			}
			if t.Name.Local == "t" {
				inText = false
			}
		}
	}

	return buf.String(), nil
}

func (e *TextExtractor) extractFromImage(data []byte) (string, error) {
	return "", fmt.Errorf("图片文件暂不支持文本提取，请将简历转换为PDF或DOCX格式后上传")
}
