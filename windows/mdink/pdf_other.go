//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func writePDF(outPath string, src []byte, title string, meta map[string]string, baseDir string) error {
	htmlDoc, err := renderHTML(src, title, baseDir, meta)
	if err != nil {
		return err
	}
	// On non-Windows, emit HTML next to the intended PDF so the pipeline can be checked.
	htmlPath := filepath.Join(filepath.Dir(outPath), filepath.Base(outPath)+".html")
	if err := os.WriteFile(htmlPath, []byte(htmlDoc), 0644); err != nil {
		return err
	}
	return fmt.Errorf("PDF 轉檔僅在 Windows 上透過 Microsoft Edge 產生（已輸出 HTML 預覽：%s）", htmlPath)
}
