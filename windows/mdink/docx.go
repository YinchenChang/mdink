package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type relSpec struct {
	ID, Type, Target, Mode string
}

type mediaPart struct {
	Name string
	Data []byte
	CT   string
}

type runStyle struct {
	bold, italic, strike, code bool
	href                       string
}

type docxBuilder struct {
	src     []byte
	baseDir string
	body    strings.Builder
	rels    []relSpec
	media   []mediaPart
	relN    int
	imgN    int
	numN    int
	quote   int
}

func writeDocx(outPath string, src []byte, title string, meta map[string]string, baseDir string) error {
	md := mdParser()
	doc := md.Parser().Parse(text.NewReader(src))
	b := &docxBuilder{src: src, baseDir: baseDir, relN: 1, numN: 1}
	b.rels = []relSpec{
		{ID: "rId1", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles", Target: "styles.xml", Mode: ""},
		{ID: "rId2", Type: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering", Target: "numbering.xml", Mode: ""},
	}
	b.relN = 2

	if title != "" && !startsWithHeading(doc) {
		b.pStyle("Heading1", title, runStyle{})
	}
	b.blocks(doc)

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	author := meta["author"]
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	files := map[string]string{
		"[Content_Types].xml":     b.contentTypes(),
		"_rels/.rels":             relsRoot(),
		"docProps/core.xml":       coreXML(title, author, now),
		"docProps/app.xml":        appXML(),
		"word/document.xml":       b.documentXML(),
		"word/styles.xml":         stylesXML(),
		"word/numbering.xml":      numberingXML(),
		"word/_rels/document.xml.rels": b.documentRels(),
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, content); err != nil {
			return err
		}
	}
	for _, m := range b.media {
		w, err := zw.Create("word/" + m.Name)
		if err != nil {
			return err
		}
		if _, err := w.Write(m.Data); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(outPath, buf.Bytes(), 0644)
}

func startsWithHeading(n ast.Node) bool {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*ast.Heading); ok {
			return true
		}
		if _, ok := c.(*ast.Paragraph); ok {
			return false
		}
	}
	return false
}

func (b *docxBuilder) nextRel(typ, target, mode string) string {
	b.relN++
	id := fmt.Sprintf("rId%d", b.relN)
	b.rels = append(b.rels, relSpec{ID: id, Type: typ, Target: target, Mode: mode})
	return id
}

func (b *docxBuilder) blocks(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		b.block(c)
	}
}

func (b *docxBuilder) block(n ast.Node) {
	switch t := n.(type) {
	case *ast.Heading:
		style := fmt.Sprintf("Heading%d", clamp(t.Level, 1, 6))
		b.openP(style, 0, false)
		b.inlines(t, runStyle{})
		b.closeP()
	case *ast.Paragraph:
		b.openP("Normal", 0, false)
		b.inlines(t, runStyle{})
		b.closeP()
	case *ast.List:
		b.list(t, 0)
	case *ast.CodeBlock, *ast.FencedCodeBlock:
		b.codeBlock(t)
	case *ast.Blockquote:
		b.quote++
		b.blocks(t)
		b.quote--
	case *ast.ThematicBreak:
		b.body.WriteString(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="D9D2C5"/></w:pBdr></w:pPr></w:p>`)
	case *ast.HTMLBlock:
	case *east.Table:
		b.table(t)
	default:
		if t.HasChildren() {
			b.blocks(t)
		}
	}
}

func (b *docxBuilder) list(list *ast.List, depth int) {
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		li, ok := item.(*ast.ListItem)
		if !ok {
			continue
		}
		b.listItem(li, list, depth)
	}
}

func (b *docxBuilder) listItem(li *ast.ListItem, parent *ast.List, depth int) {
	ordered := parent.IsOrdered()
	numID := 1
	if ordered {
		numID = 2
	}
	marker := ""
	for c := li.FirstChild(); c != nil; c = c.NextSibling() {
		if cb, ok := c.(*east.TaskCheckBox); ok {
			if cb.IsChecked {
				marker = "☑ "
			} else {
				marker = "☐ "
			}
			continue
		}
		if nested, ok := c.(*ast.List); ok {
			b.list(nested, depth+1)
			continue
		}
		style := "ListBullet"
		if ordered {
			style = "ListNumber"
		}
		b.openP(style, depth, true)
		b.body.WriteString(fmt.Sprintf(
			`<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`,
			clamp(depth, 0, 8), numID,
		))
		b.closePPr()
		if marker != "" {
			b.run(marker, runStyle{})
			marker = ""
		}
		if p, ok := c.(*ast.Paragraph); ok {
			b.inlines(p, runStyle{})
		} else if h, ok := c.(*ast.Heading); ok {
			b.inlines(h, runStyle{})
		} else if c.Kind() == ast.KindTextBlock {
			b.inlines(c, runStyle{})
		} else {
			b.block(c)
		}
		b.closeP()
	}
}

func (b *docxBuilder) codeBlock(n ast.Node) {
	var raw []byte
	switch t := n.(type) {
	case *ast.FencedCodeBlock:
		raw = b.linesOf(t.Lines())
	case *ast.CodeBlock:
		raw = b.linesOf(t.Lines())
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return
	}
	for _, line := range strings.Split(text, "\n") {
		b.body.WriteString(`<w:p><w:pPr><w:pStyle w:val="Code"/><w:shd w:val="clear" w:fill="F3EFE6"/></w:pPr>`)
		b.run(line, runStyle{code: true})
		b.closeP()
	}
}

func (b *docxBuilder) linesOf(ls *text.Segments) []byte {
	if ls == nil {
		return nil
	}
	return ls.Value(b.src)
}

func (b *docxBuilder) table(tbl *east.Table) {
	b.body.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="5000" w:type="pct"/><w:tblBorders>`)
	b.body.WriteString(`<w:top w:val="single" w:sz="4" w:color="D9D2C5"/>`)
	b.body.WriteString(`<w:left w:val="single" w:sz="4" w:color="D9D2C5"/>`)
	b.body.WriteString(`<w:bottom w:val="single" w:sz="4" w:color="D9D2C5"/>`)
	b.body.WriteString(`<w:right w:val="single" w:sz="4" w:color="D9D2C5"/>`)
	b.body.WriteString(`<w:insideH w:val="single" w:sz="4" w:color="D9D2C5"/>`)
	b.body.WriteString(`<w:insideV w:val="single" w:sz="4" w:color="D9D2C5"/>`)
	b.body.WriteString(`</w:tblBorders></w:tblPr>`)
	header := true
	for row := tbl.FirstChild(); row != nil; row = row.NextSibling() {
		tr, ok := row.(*east.TableRow)
		if !ok {
			if hg, ok := row.(*east.TableHeader); ok {
				for r := hg.FirstChild(); r != nil; r = r.NextSibling() {
					if rr, ok := r.(*east.TableRow); ok {
						b.tableRow(rr, true)
					}
				}
				header = false
				continue
			}
			continue
		}
		b.tableRow(tr, header)
		header = false
	}
	b.body.WriteString(`</w:tbl>`)
}

func (b *docxBuilder) tableRow(row *east.TableRow, header bool) {
	b.body.WriteString(`<w:tr>`)
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		tc, ok := cell.(*east.TableCell)
		if !ok {
			continue
		}
		b.body.WriteString(`<w:tc><w:tcPr>`)
		if header {
			b.body.WriteString(`<w:shd w:val="clear" w:fill="F3EFE6"/>`)
		}
		b.body.WriteString(`</w:tcPr>`)
		b.openP("Normal", 0, false)
		st := runStyle{bold: header}
		b.inlines(tc, st)
		b.closeP()
		b.body.WriteString(`</w:tc>`)
	}
	b.body.WriteString(`</w:tr>`)
}

func (b *docxBuilder) inlines(n ast.Node, st runStyle) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			s := string(t.Segment.Value(b.src))
			b.run(s, st)
			if t.HardLineBreak() || t.SoftLineBreak() {
				b.body.WriteString(`<w:r><w:br/></w:r>`)
			}
		case *ast.String:
			b.run(string(t.Value), st)
		case *ast.CodeSpan:
			st2 := st
			st2.code = true
			b.run(string(t.Text(b.src)), st2)
		case *ast.Emphasis:
			st2 := st
			if t.Level >= 2 {
				st2.bold = true
			} else {
				st2.italic = true
			}
			b.inlines(t, st2)
		case *east.Strikethrough:
			st2 := st
			st2.strike = true
			b.inlines(t, st2)
		case *ast.Link:
			st2 := st
			st2.href = string(t.Destination)
			rid := b.nextRel(
				"http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink",
				st2.href,
				"External",
			)
			b.body.WriteString(`<w:hyperlink r:id="` + rid + `">`)
			b.inlines(t, st2)
			b.body.WriteString(`</w:hyperlink>`)
		case *ast.AutoLink:
			u := string(t.URL(b.src))
			st2 := st
			st2.href = u
			rid := b.nextRel(
				"http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink",
				u,
				"External",
			)
			b.body.WriteString(`<w:hyperlink r:id="` + rid + `">`)
			b.run(u, st2)
			b.body.WriteString(`</w:hyperlink>`)
		case *ast.Image:
			b.image(string(t.Destination), string(t.Text(b.src)))
		case *east.TaskCheckBox:
			if t.IsChecked {
				b.run("☑ ", st)
			} else {
				b.run("☐ ", st)
			}
		default:
			if t.HasChildren() {
				b.inlines(t, st)
			}
		}
	}
}

func (b *docxBuilder) image(src, alt string) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "data:") {
		b.run("["+alt+"]", runStyle{italic: true})
		return
	}
	p := src
	if !filepath.IsAbs(p) {
		p = filepath.Join(b.baseDir, filepath.FromSlash(src))
	}
	data, err := os.ReadFile(p)
	if err != nil {
		b.run("["+alt+"]", runStyle{italic: true})
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		b.run("["+alt+"]", runStyle{italic: true})
		return
	}
	b.imgN++
	ext := format
	if ext == "jpeg" {
		ext = "jpg"
	}
	name := fmt.Sprintf("media/image%d.%s", b.imgN, ext)
	ct := "image/" + format
	if format == "jpg" {
		ct = "image/jpeg"
	}
	b.media = append(b.media, mediaPart{Name: name, Data: data, CT: ct})
	rid := b.nextRel(
		"http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
		name,
		"",
	)
	w, h := int64(cfg.Width), int64(cfg.Height)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	const max = int64(6.3 * 914400)
	cx := w * 9525
	cy := h * 9525
	if cx > max {
		cy = cy * max / cx
		cx = max
	}
	b.body.WriteString(`<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0">`)
	b.body.WriteString(fmt.Sprintf(`<wp:extent cx="%d" cy="%d"/>`, cx, cy))
	b.body.WriteString(`<wp:docPr id="` + strconv.Itoa(b.imgN) + `" name="Picture ` + strconv.Itoa(b.imgN) + `"/>`)
	b.body.WriteString(`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`)
	b.body.WriteString(`<pic:pic>`)
	b.body.WriteString(`<pic:nvPicPr><pic:cNvPr id="0" name="` + xmlEscape(filepath.Base(p)) + `"/><pic:cNvPicPr/></pic:nvPicPr>`)
	b.body.WriteString(`<pic:blipFill><a:blip r:embed="` + rid + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`)
	b.body.WriteString(fmt.Sprintf(`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>`, cx, cy))
	b.body.WriteString(`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r>`)
}

func (b *docxBuilder) pStyle(style, text string, st runStyle) {
	b.openP(style, 0, false)
	b.run(text, st)
	b.closeP()
}

func (b *docxBuilder) openP(style string, _ int, keepPrOpen bool) {
	b.body.WriteString(`<w:p><w:pPr>`)
	if style != "" && style != "Normal" {
		b.body.WriteString(`<w:pStyle w:val="` + style + `"/>`)
	}
	if b.quote > 0 {
		b.body.WriteString(`<w:ind w:left="720"/><w:pBdr><w:left w:val="single" w:sz="12" w:space="8" w:color="3F4F46"/></w:pBdr>`)
	}
	if !keepPrOpen {
		b.closePPr()
	}
}

func (b *docxBuilder) closePPr() {
	b.body.WriteString(`</w:pPr>`)
}

func (b *docxBuilder) closeP() {
	b.body.WriteString(`</w:p>`)
}

func (b *docxBuilder) run(s string, st runStyle) {
	if s == "" {
		return
	}
	b.body.WriteString(`<w:r><w:rPr>`)
	if st.code {
		b.body.WriteString(`<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas" w:eastAsia="Microsoft JhengHei"/>`)
		b.body.WriteString(`<w:sz w:val="20"/><w:shd w:val="clear" w:fill="F3EFE6"/>`)
	}
	if st.bold {
		b.body.WriteString(`<w:b/><w:bCs/>`)
	}
	if st.italic {
		b.body.WriteString(`<w:i/><w:iCs/>`)
	}
	if st.strike {
		b.body.WriteString(`<w:strike/>`)
	}
	if st.href != "" {
		b.body.WriteString(`<w:color w:val="2C4A3E"/><w:u w:val="single"/>`)
	}
	b.body.WriteString(`</w:rPr><w:t xml:space="preserve">`)
	b.body.WriteString(xmlEscape(s))
	b.body.WriteString(`</w:t></w:r>`)
}

func (b *docxBuilder) documentXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas" ` +
		`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
		`xmlns:o="urn:schemas-microsoft-com:office:office" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" ` +
		`xmlns:v="urn:schemas-microsoft-com:vml" ` +
		`xmlns:wp14="http://schemas.microsoft.com/office/word/2010/wordprocessingDrawing" ` +
		`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
		`xmlns:w10="urn:schemas-microsoft-com:office:word" ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" ` +
		`xmlns:wne="http://schemas.microsoft.com/office/wordml/2006/wordml" ` +
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
		`xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" ` +
		`mc:Ignorable="w14 wp14">` +
		`<w:body>` + b.body.String() +
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708"/></w:sectPr>` +
		`</w:body></w:document>`
}

func (b *docxBuilder) documentRels() string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, r := range b.rels {
		sb.WriteString(`<Relationship Id="` + r.ID + `" Type="` + r.Type + `" Target="` + xmlEscape(r.Target) + `"`)
		if r.Mode != "" {
			sb.WriteString(` TargetMode="` + r.Mode + `"`)
		}
		sb.WriteString(`/>`)
	}
	sb.WriteString(`</Relationships>`)
	return sb.String()
}

func (b *docxBuilder) contentTypes() string {
	var extras strings.Builder
	seen := map[string]bool{}
	for _, m := range b.media {
		ext := strings.TrimPrefix(filepath.Ext(m.Name), ".")
		if seen[ext] {
			continue
		}
		seen[ext] = true
		ct := m.CT
		extras.WriteString(`<Default Extension="` + ext + `" ContentType="` + ct + `"/>`)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		extras.String() +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
		`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`</Types>`
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func relsRoot() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func coreXML(title, author, now string) string {
	if author == "" {
		author = "墨轉 MdInk"
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" ` +
		`xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:title>` + xmlEscape(title) + `</dc:title>` +
		`<dc:creator>` + xmlEscape(author) + `</dc:creator>` +
		`<cp:lastModifiedBy>` + xmlEscape(author) + `</cp:lastModifiedBy>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>` +
		`</cp:coreProperties>`
}

func appXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" ` +
		`xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">` +
		`<Application>墨轉 MdInk</Application></Properties>`
}

func stylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:docDefaults><w:rPrDefault><w:rPr>` +
		`<w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Microsoft JhengHei" w:cs="Calibri"/>` +
		`<w:sz w:val="24"/><w:szCs w:val="24"/>` +
		`</w:rPr></w:rPrDefault>` +
		`<w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault>` +
		`</w:docDefaults>` +
		headingStyle(1, 44) + headingStyle(2, 36) + headingStyle(3, 32) +
		headingStyle(4, 28) + headingStyle(5, 26) + headingStyle(6, 24) +
		`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/>` +
		`<w:pPr><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>` +
		`<w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas" w:eastAsia="Microsoft JhengHei"/><w:sz w:val="20"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="ListBullet"><w:name w:val="List Bullet"/><w:basedOn w:val="Normal"/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="ListNumber"><w:name w:val="List Number"/><w:basedOn w:val="Normal"/></w:style>` +
		`</w:styles>`
}

func headingStyle(level, sz int) string {
	return fmt.Sprintf(
		`<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/>`+
			`<w:basedOn w:val="Normal"/><w:uiPriority w:val="%d"/><w:qFormat/>`+
			`<w:pPr><w:keepNext/><w:spacing w:before="280" w:after="80"/></w:pPr>`+
			`<w:rPr><w:b/><w:bCs/><w:sz w:val="%d"/><w:szCs w:val="%d"/>`+
			`<w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Microsoft JhengHei"/></w:rPr></w:style>`,
		level, level, level, sz, sz,
	)
}

func numberingXML() string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	sb.WriteString(`<w:abstractNum w:abstractNumId="0"><w:multiLevelType w:val="hybridMultilevel"/>`)
	bullets := []string{"●", "○", "■"}
	for i := 0; i < 9; i++ {
		b := bullets[i%3]
		sb.WriteString(fmt.Sprintf(
			`<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="bullet"/>`+
				`<w:lvlText w:val="%s"/><w:lvlJc w:val="left"/>`+
				`<w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr>`+
				`<w:rPr><w:rFonts w:ascii="Segoe UI Symbol" w:hAnsi="Segoe UI Symbol" w:eastAsia="Segoe UI Symbol"/></w:rPr>`+
				`</w:lvl>`, i, xmlEscape(b), 720*(i+1),
		))
	}
	sb.WriteString(`</w:abstractNum>`)
	sb.WriteString(`<w:abstractNum w:abstractNumId="1"><w:multiLevelType w:val="hybridMultilevel"/>`)
	fmts := []string{"decimal", "lowerLetter", "lowerRoman"}
	texts := []string{"%1.", "%2.", "%3."}
	for i := 0; i < 9; i++ {
		nf := fmts[i%3]
		tx := texts[i%3]
		if i >= 3 {
			tx = fmt.Sprintf("%%%d.", i+1)
		}
		sb.WriteString(fmt.Sprintf(
			`<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/>`+
				`<w:lvlText w:val="%s"/><w:lvlJc w:val="left"/>`+
				`<w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`,
			i, nf, tx, 720*(i+1),
		))
	}
	sb.WriteString(`</w:abstractNum>`)
	sb.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)
	sb.WriteString(`<w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>`)
	sb.WriteString(`</w:numbering>`)
	return sb.String()
}
