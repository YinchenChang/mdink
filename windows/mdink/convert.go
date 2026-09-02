package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

func mdParser() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.Typographer,
			extension.DefinitionList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)
}

func convertOne(path string, formats []string) ([]string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(src)) == 0 {
		return nil, fmt.Errorf("檔案是空的")
	}
	meta, body := splitFrontMatter(src)
	baseDir := filepath.Dir(path)
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	title := meta["title"]
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	var outs []string
	for _, f := range formats {
		switch f {
		case "docx":
			out := stem + ".docx"
			if err := writeDocx(out, body, title, meta, baseDir); err != nil {
				return outs, err
			}
			outs = append(outs, out)
		case "pdf":
			out := stem + ".pdf"
			if err := writePDF(out, body, title, meta, baseDir); err != nil {
				return outs, err
			}
			outs = append(outs, out)
		default:
			return outs, fmt.Errorf("不支援的格式：%s", f)
		}
	}
	return outs, nil
}

func splitFrontMatter(src []byte) (map[string]string, []byte) {
	s := string(src)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return map[string]string{}, src
	}
	rest := s[3:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return map[string]string{}, src
	}
	fm := rest[:idx]
	body := rest[idx+4:]
	body = strings.TrimPrefix(body, "\r")
	body = strings.TrimPrefix(body, "\n")
	meta := map[string]string{}
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		meta[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return meta, []byte(body)
}
