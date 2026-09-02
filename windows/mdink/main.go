package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	appName    = "墨轉 MdInk"
	appVersion = "1.0.1"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			alertError(fmt.Sprintf("程式發生未預期錯誤：%v", r))
			os.Exit(1)
		}
	}()

	to, notify, doInstallFlag, doUninstallFlag, quiet, files := parseArgs(os.Args[1:])

	if doUninstallFlag {
		if err := runUninstall(quiet); err != nil {
			alertError("解除安裝失敗：\n" + err.Error())
			os.Exit(1)
		}
		return
	}

	if doInstallFlag {
		if err := doInstall(); err != nil {
			alertError("安裝失敗：\n" + err.Error())
			os.Exit(1)
		}
		alertInstalled()
		return
	}

	if len(files) == 0 {
		handleNoArgs()
		return
	}

	if len(to) == 0 {
		choice := chooseFormat()
		switch choice {
		case btnDocx:
			to = []string{"docx"}
		case btnPDF:
			to = []string{"pdf"}
		case btnBoth:
			to = []string{"docx", "pdf"}
		default:
			return
		}
	}

	outputs, errs := convertFiles(files, to)
	if notify || len(errs) > 0 || len(outputs) > 0 {
		reportResult(outputs, errs)
	}
	if len(errs) > 0 && len(outputs) == 0 {
		os.Exit(1)
	}
}

func parseArgs(args []string) (to []string, notify, install, uninstall, quiet bool, files []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--to" && i+1 < len(args):
			i++
			for _, p := range strings.Split(args[i], ",") {
				p = strings.ToLower(strings.TrimSpace(p))
				if p == "doc" || p == "word" {
					p = "docx"
				}
				if p != "" {
					to = append(to, p)
				}
			}
		case strings.HasPrefix(a, "--to="):
			for _, p := range strings.Split(strings.TrimPrefix(a, "--to="), ",") {
				p = strings.ToLower(strings.TrimSpace(p))
				if p == "doc" || p == "word" {
					p = "docx"
				}
				if p != "" {
					to = append(to, p)
				}
			}
		case a == "--notify":
			notify = true
		case a == "--install":
			install = true
		case a == "--uninstall":
			uninstall = true
		case a == "--quiet" || a == "/S" || a == "/silent":
			quiet = true
		case a == "--help" || a == "-h":
			alertInfo(helpText())
			os.Exit(0)
		case strings.HasPrefix(a, "-"):
			alertError("未知參數：" + a)
			os.Exit(2)
		default:
			files = append(files, a)
		}
	}
	return
}

func handleNoArgs() {
	installed := isInstalled()
	fromInstall := runningFromInstallDir()

	if !installed {
		if askYesNo(
			"安裝墨轉",
			"將 Markdown 轉成 Word 或 PDF，並加入檔案總管右鍵選單。",
			"安裝在目前使用者帳戶下，不需要系統管理員權限。\n\n安裝後：對 .md 檔按右鍵 → 顯示更多選項 → 墨轉 Markdown。",
		) {
			if err := doInstall(); err != nil {
				alertError("安裝失敗：\n" + err.Error())
				os.Exit(1)
			}
			alertInstalled()
		}
		return
	}

	if !fromInstall {
		choice := chooseManage()
		switch choice {
		case btnRepair:
			if err := doInstall(); err != nil {
				alertError("修復失敗：\n" + err.Error())
				os.Exit(1)
			}
			alertInfo("已重新註冊右鍵選單，墨轉可以使用了。")
		case btnUninstall:
			if !confirmUninstall() {
				return
			}
			if err := runUninstall(false); err != nil {
				alertError("解除安裝失敗：\n" + err.Error())
				os.Exit(1)
			}
		}
		return
	}

	choice := chooseInstalled()
	if choice == btnUninstall {
		if !confirmUninstall() {
			return
		}
		if err := runUninstall(false); err != nil {
			alertError("解除安裝失敗：\n" + err.Error())
			os.Exit(1)
		}
	}
}

func runUninstall(quiet bool) error {
	if err := doUninstall(); err != nil {
		return err
	}
	if !quiet {
		alertInfo("已從這台電腦移除墨轉。\n\n右鍵選單、開始功能表捷徑與程式檔都會清掉。之後可在設定 → 應用程式確認清單裡已沒有「墨轉 MdInk」。")
	}
	finishUninstall()
	return nil
}

func helpText() string {
	return appName + "  " + appVersion + `

把 .md 檔轉成 Word（.docx）或 PDF。

【檔案總管】
對 Markdown 檔按右鍵 →「顯示更多選項」
→ 墨轉 Markdown → 轉成 Word / 轉成 PDF。

Windows 11 精簡右鍵選單無法由一般程式直接加入項目，
這是系統限制，不是安裝失敗。可改用：
• Shift + 右鍵
• 把 .md 拖放到「墨轉」捷徑上

【解除安裝】
設定 → 應用程式 → 已安裝的應用程式 → 墨轉 MdInk → 解除安裝
開始功能表 → 墨轉 MdInk → 解除安裝
或再次執行安裝檔並選擇解除安裝。`
}

func convertFiles(files []string, formats []string) (outputs []string, errs []string) {
	seen := map[string]bool{}
	var clean []string
	for _, f := range formats {
		if !seen[f] {
			seen[f] = true
			clean = append(clean, f)
		}
	}
	for _, raw := range files {
		path, err := filepath.Abs(raw)
		if err != nil {
			errs = append(errs, raw+"：路徑無效")
			continue
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".markdown" && ext != ".mdown" && ext != ".mkd" {
			errs = append(errs, filepath.Base(path)+"：不是 Markdown 檔")
			continue
		}
		outs, err := convertOne(path, clean)
		if err != nil {
			errs = append(errs, filepath.Base(path)+"： "+err.Error())
		}
		outputs = append(outputs, outs...)
	}
	return
}

func reportResult(outputs []string, errs []string) {
	var b strings.Builder
	if len(outputs) > 0 {
		b.WriteString("已輸出：\n")
		for _, o := range outputs {
			b.WriteString("• ")
			b.WriteString(o)
			b.WriteString("\n")
		}
	}
	if len(errs) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("未完成：\n")
		for _, e := range errs {
			b.WriteString("• ")
			b.WriteString(e)
			b.WriteString("\n")
		}
	}
	if len(outputs) == 0 {
		alertError(strings.TrimSpace(b.String()))
		return
	}
	choice := afterConvert(strings.TrimSpace(b.String()), len(errs) > 0)
	if len(outputs) == 0 {
		return
	}
	first := outputs[0]
	switch choice {
	case btnOpenFile:
		openPath(first)
	case btnOpenFolder:
		openFolderSelect(first)
	}
}
