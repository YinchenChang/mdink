//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func writePDF(outPath string, src []byte, title string, meta map[string]string, baseDir string) error {
	edge := findEdge()
	if edge == "" {
		return fmt.Errorf("找不到 Microsoft Edge，無法產生 PDF（Windows 11 應已內建）")
	}
	htmlDoc, err := renderHTML(src, title, baseDir, meta)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "mdink-pdf-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	htmlPath := filepath.Join(tmp, "doc.html")
	pdfTmp := filepath.Join(tmp, "doc.pdf")
	if err := os.WriteFile(htmlPath, []byte(htmlDoc), 0644); err != nil {
		return err
	}

	fileURL := pathToFileURL(htmlPath)
	cmd := exec.Command(edge,
		"--headless=new",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--hide-scrollbars",
		"--no-pdf-header-footer",
		"--print-to-pdf="+pdfTmp,
		fileURL,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := runWithTimeout(cmd, 90*time.Second); err != nil {
		return fmt.Errorf("Edge 產生 PDF 失敗：%w", err)
	}
	if st, err := os.Stat(pdfTmp); err != nil || st.Size() < 32 {
		return fmt.Errorf("PDF 沒有成功寫出")
	}
	data, err := os.ReadFile(pdfTmp)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0644)
}

func findEdge() string {
	var cands []string
	for _, env := range []string{"PROGRAMFILES", "PROGRAMFILES(X86)", "LOCALAPPDATA"} {
		root := os.Getenv(env)
		if root == "" {
			continue
		}
		cands = append(cands, filepath.Join(root, `Microsoft\Edge\Application\msedge.exe`))
	}
	cands = append(cands,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	)
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	if p, err := exec.LookPath("msedge"); err == nil {
		return p
	}
	return ""
}

func pathToFileURL(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	abs = filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	return "file://" + abs
}

func runWithTimeout(cmd *exec.Cmd, d time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		_ = cmd.Process.Kill()
		return fmt.Errorf("逾時")
	}
}
