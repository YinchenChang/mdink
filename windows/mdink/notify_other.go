//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	btnCancel     = 0
	btnDocx       = 101
	btnPDF        = 102
	btnBoth       = 103
	btnRepair     = 104
	btnUninstall  = 105
	btnOpenFile   = 106
	btnOpenFolder = 107
	btnOK         = 1
)

func alertInfo(text string)  { fmt.Fprintln(os.Stderr, text) }
func alertError(text string) { fmt.Fprintln(os.Stderr, text) }
func alertInstalled()        { alertInfo("安裝完成（僅 Windows）") }
func askYesNo(string, string, string) bool { return false }
func chooseFormat() int                    { return btnDocx }
func chooseManage() int                    { return btnCancel }
func chooseInstalled() int                 { return btnOK }
func confirmUninstall() bool               { return true }
func afterConvert(detail string, hadErr bool) int {
	if hadErr {
		fmt.Fprintln(os.Stderr, detail)
	} else {
		fmt.Println(detail)
	}
	return btnOK
}
func openPath(p string) { _ = exec.Command("xdg-open", p).Start() }
func openFolderSelect(p string) {
	_ = exec.Command("xdg-open", filepath.Dir(p)).Start()
}
