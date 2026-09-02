//go:build windows

package main


import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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

const (
	tdfAllowCancellation = 0x0008
	tdfUseCommandLinks   = 0x0010
	tdfSizeToContent     = 0x01000000
	tdInformationIcon    = ^uintptr(2)
	tdErrorIcon          = ^uintptr(1)
)

type tdButton struct {
	id   int32
	text *uint16
}

type tdConfig struct {
	cbSize                  uint32
	hwndParent              uintptr
	hInstance               uintptr
	dwFlags                 uint32
	dwCommonButtons         uint32
	pszWindowTitle          *uint16
	pszMainIcon             uintptr
	pszMainInstruction      *uint16
	pszContent              *uint16
	cButtons                uint32
	pButtons                uintptr
	nDefaultButton          int32
	cRadioButtons           uint32
	pRadioButtons           uintptr
	nDefaultRadioButton     int32
	pszVerificationText     *uint16
	pszExpandedInformation  *uint16
	pszExpandedControlText  *uint16
	pszCollapsedControlText *uint16
	pszFooterIcon           uintptr
	pszFooter               *uint16
	pfCallback              uintptr
	lpCallbackData          uintptr
	cxWidth                 uint32
}

var (
	comctl32               = windows.NewLazySystemDLL("comctl32.dll")
	procTaskDialogIndirect = comctl32.NewProc("TaskDialogIndirect")
	procInitCCC            = comctl32.NewProc("InitCommonControlsEx")
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procMessageBoxW        = user32.NewProc("MessageBoxW")
	initedCC               bool
)

func initCommonControls() {
	if initedCC {
		return
	}
	type icc struct {
		dwSize, dwICC uint32
	}
	v := icc{dwSize: 8, dwICC: 0x000040FF}
	_, _, _ = procInitCCC.Call(uintptr(unsafe.Pointer(&v)))
	initedCC = true
}

func utf16ptr(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func taskDialog(title, main, content string, icon uintptr, flags uint32, buttons []tdButton, defaultID int32) (int, bool) {
	initCommonControls()
	if procTaskDialogIndirect.Find() != nil {
		return 0, false
	}
	cfg := tdConfig{
		hwndParent:         0,
		dwFlags:            flags | tdfAllowCancellation | tdfSizeToContent,
		pszWindowTitle:     utf16ptr(title),
		pszMainIcon:        icon,
		pszMainInstruction: utf16ptr(main),
		pszContent:         utf16ptr(content),
		cButtons:           uint32(len(buttons)),
		nDefaultButton:     defaultID,
	}
	cfg.cbSize = uint32(unsafe.Sizeof(cfg))
	if len(buttons) > 0 {
		cfg.pButtons = uintptr(unsafe.Pointer(&buttons[0]))
	}
	var clicked int32
	r, _, _ := procTaskDialogIndirect.Call(
		uintptr(unsafe.Pointer(&cfg)),
		uintptr(unsafe.Pointer(&clicked)),
		0,
		0,
	)
	if r != 0 {
		return 0, false
	}
	return int(clicked), true
}

func messageBox(title, text string, flags uint) int {
	r, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(utf16ptr(text))),
		uintptr(unsafe.Pointer(utf16ptr(title))),
		uintptr(flags),
	)
	return int(r)
}

func alertInfo(text string) {
	if _, ok := taskDialog(appName, appName, text, tdInformationIcon, 0, []tdButton{
		{id: btnOK, text: utf16ptr("確定")},
	}, btnOK); !ok {
		messageBox(appName, text, 0x40)
	}
}

func alertError(text string) {
	if _, ok := taskDialog(appName, "無法完成", text, tdErrorIcon, 0, []tdButton{
		{id: btnOK, text: utf16ptr("確定")},
	}, btnOK); !ok {
		messageBox(appName, text, 0x10)
	}
}

func alertInstalled() {
	alertInfo("安裝完成。\n\n在 .md 檔上按右鍵 →「顯示更多選項」→「墨轉 Markdown」。\n\nWindows 11 的精簡右鍵選單無法由一般程式直接加入項目；這是系統限制。可改用 Shift+右鍵，或把檔案拖到開始功能表的墨轉捷徑上。")
}

func askYesNo(title, main, content string) bool {
	id, ok := taskDialog(title, main, content, tdInformationIcon, tdfUseCommandLinks, []tdButton{
		{id: btnOK, text: utf16ptr("安裝\n複製到使用者資料夾並註冊右鍵選單")},
		{id: btnCancel, text: utf16ptr("取消")},
	}, btnOK)
	if ok {
		return id == btnOK
	}
	return messageBox(title, main+"\n\n"+content, 0x24) == 6
}

func chooseFormat() int {
	id, ok := taskDialog(appName, "要轉成哪一種格式？", "輸出檔會放在 Markdown 同一個資料夾。", tdInformationIcon, tdfUseCommandLinks, []tdButton{
		{id: btnDocx, text: utf16ptr("轉成 Word 文件 (.docx)\n可用 Word、Google 文件、WPS 開啟")},
		{id: btnPDF, text: utf16ptr("轉成 PDF\n用系統 Edge 排版，中文與圖片較完整")},
		{id: btnBoth, text: utf16ptr("兩種格式都轉")},
	}, btnDocx)
	if ok {
		return id
	}
	r := messageBox(appName, "是 = Word (.docx)\n否 = PDF\n取消 = 結束", 0x23)
	switch r {
	case 6:
		return btnDocx
	case 7:
		return btnPDF
	default:
		return btnCancel
	}
}

func chooseManage() int {
	id, ok := taskDialog(appName, "墨轉已經安裝在這台電腦", "可以重新註冊右鍵選單，或完全移除。", tdInformationIcon, tdfUseCommandLinks, []tdButton{
		{id: btnRepair, text: utf16ptr("修復 / 重新註冊右鍵選單")},
		{id: btnUninstall, text: utf16ptr("解除安裝\n移除右鍵選單、捷徑與程式")},
		{id: btnCancel, text: utf16ptr("關閉")},
	}, btnRepair)
	if ok {
		return id
	}
	r := messageBox(appName, "是 = 修復\n否 = 解除安裝\n取消 = 關閉", 0x23)
	switch r {
	case 6:
		return btnRepair
	case 7:
		return btnUninstall
	default:
		return btnCancel
	}
}

func chooseInstalled() int {
	id, ok := taskDialog(appName, appName+" 已安裝", helpText(), tdInformationIcon, tdfUseCommandLinks, []tdButton{
		{id: btnOK, text: utf16ptr("關閉")},
		{id: btnUninstall, text: utf16ptr("解除安裝\n從這台電腦移除墨轉")},
	}, btnOK)
	if ok {
		return id
	}
	if messageBox(appName, helpText()+"\n\n要解除安裝嗎？\n是 = 解除安裝，否 = 關閉", 0x24) == 6 {
		return btnUninstall
	}
	return btnOK
}

func confirmUninstall() bool {
	id, ok := taskDialog(appName, "要解除安裝墨轉嗎？", "會移除檔案總管右鍵選單、開始功能表捷徑，以及程式本身。轉換過的 Word / PDF 不會刪除。", tdInformationIcon, tdfUseCommandLinks, []tdButton{
		{id: btnUninstall, text: utf16ptr("解除安裝")},
		{id: btnCancel, text: utf16ptr("取消")},
	}, btnCancel)
	if ok {
		return id == btnUninstall
	}
	return messageBox(appName, "確定要解除安裝墨轉嗎？", 0x24) == 6
}

func afterConvert(detail string, hadErr bool) int {
	icon := tdInformationIcon
	main := "轉換完成"
	if hadErr {
		icon = tdErrorIcon
		main = "部分檔案轉換完成"
	}
	id, ok := taskDialog(appName, main, detail, icon, tdfUseCommandLinks, []tdButton{
		{id: btnOpenFile, text: utf16ptr("開啟檔案")},
		{id: btnOpenFolder, text: utf16ptr("開啟所在資料夾")},
		{id: btnOK, text: utf16ptr("關閉")},
	}, btnOpenFile)
	if ok {
		return id
	}
	messageBox(appName, detail, 0x40)
	return btnOK
}

func openPath(p string) {
	_ = exec.Command("cmd", "/c", "start", "", p).Start()
}

func openFolderSelect(p string) {
	cmd := exec.Command("explorer", "/select,", p)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

func logf(format string, args ...any) {
	dir := installDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "mdink.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(fmtLog(format, args...))
}

func fmtLog(format string, args ...any) string {
	return fmtS(format, args...) + "\n"
}

func fmtS(format string, args ...any) string {
	return sprintf(format, args...)
}
