package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func renderHTML(src []byte, title, baseDir string, meta map[string]string) (string, error) {
	var buf bytes.Buffer
	if err := mdParser().Convert(src, &buf); err != nil {
		return "", err
	}
	body := rewriteImages(buf.String(), baseDir)
	sub := ""
	if a := meta["author"]; a != "" {
		sub += `<div class="meta">` + html.EscapeString(a) + `</div>`
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-Hant">
<head>
<meta charset="utf-8"/>
<title>%s</title>
<style>
  @page { size: A4; margin: 16mm 16mm 18mm; }
  * { box-sizing: border-box; }
  html { font-size: 11pt; }
  body {
    margin: 0;
    color: #1c1a16;
    font-family: "Microsoft JhengHei", "PingFang TC", "Noto Sans TC", "Segoe UI", sans-serif;
    line-height: 1.7;
  }
  h1,h2,h3,h4,h5,h6 {
    font-weight: 600;
    line-height: 1.3;
    margin: 1.35em 0 0.45em;
    letter-spacing: 0.01em;
  }
  h1 { font-size: 1.7rem; border-bottom: 1px solid #d9d2c5; padding-bottom: 0.28em; margin-top: 0; }
  h2 { font-size: 1.32rem; }
  h3 { font-size: 1.12rem; }
  p { margin: 0.7em 0; }
  a { color: #2c4a3e; }
  hr { border: 0; border-top: 1px solid #d9d2c5; margin: 1.4em 0; }
  blockquote {
    margin: 0.9em 0;
    padding: 0.15em 0 0.15em 1em;
    border-left: 3px solid #3f4f46;
    color: #5c574e;
  }
  code, pre {
    font-family: "Cascadia Code", "Consolas", "Sarasa Mono TC", "Noto Sans Mono CJK TC", monospace;
  }
  code {
    background: #f3efe6;
    padding: 0.1em 0.35em;
    border-radius: 4px;
    font-size: 0.9em;
  }
  pre {
    background: #f3efe6;
    padding: 12px 14px;
    border-radius: 8px;
    overflow: auto;
    font-size: 0.86em;
    line-height: 1.5;
  }
  pre code { background: none; padding: 0; }
  table { border-collapse: collapse; width: 100%%; margin: 1em 0; }
  th, td { border: 1px solid #d9d2c5; padding: 6px 10px; text-align: left; vertical-align: top; }
  th { background: #f3efe6; font-weight: 600; }
  img { max-width: 100%%; height: auto; }
  ul, ol { padding-left: 1.4em; }
  li { margin: 0.2em 0; }
  .title { margin-bottom: 1.5em; }
  .meta { color: #6f6a60; font-size: 0.92rem; margin-top: 0.3em; }
</style>
</head>
<body>
<div class="title">
  <h1>%s</h1>
  %s
</div>
%s
</body>
</html>`, html.EscapeString(title), html.EscapeString(title), sub, body), nil
}

var imgSrcRe = regexp.MustCompile(`(?i)(<img\b[^>]*?\bsrc=")([^"]+)(")`)

func rewriteImages(htmlBody, baseDir string) string {
	return imgSrcRe.ReplaceAllStringFunc(htmlBody, func(m string) string {
		parts := imgSrcRe.FindStringSubmatch(m)
		if len(parts) != 4 {
			return m
		}
		src := htmlUnescape(parts[2])
		if strings.HasPrefix(src, "data:") || strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			return m
		}
		data, mime, err := readAsDataURI(baseDir, src)
		if err != nil {
			return m
		}
		return parts[1] + "data:" + mime + ";base64," + data + parts[3]
	})
}

func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&"+"amp;", "&")
	s = strings.ReplaceAll(s, "&"+"lt;", "<")
	s = strings.ReplaceAll(s, "&"+"gt;", ">")
	s = strings.ReplaceAll(s, "&"+"quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

func readAsDataURI(baseDir, src string) (string, string, error) {
	p := src
	if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, filepath.FromSlash(src))
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", "", err
	}
	if len(b) > 12<<20 {
		return "", "", fmt.Errorf("圖片太大")
	}
	mime := http.DetectContentType(b)
	if mime == "application/octet-stream" {
		switch strings.ToLower(filepath.Ext(p)) {
		case ".png":
			mime = "image/png"
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".gif":
			mime = "image/gif"
		case ".webp":
			mime = "image/webp"
		case ".svg":
			mime = "image/svg+xml"
		}
	}
	return base64.StdEncoding.EncodeToString(b), mime, nil
}
