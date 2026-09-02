//go:build windows

package main



import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, "Programs", "MdInk")
}

func installedExe() string {
	return filepath.Join(installDir(), "mdink.exe")
}

func isInstalled() bool {
	st, err := os.Stat(installedExe())
	return err == nil && !st.IsDir()
}

func runningFromInstallDir() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	self, _ = filepath.EvalSymlinks(self)
	inst, _ = filepath.EvalSymlinks(installedExe())
	return strings.EqualFold(filepath.Clean(self), filepath.Clean(inst))
}

func doInstall() error {
	dir := installDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dest := installedExe()
	if !sameFile(self, dest) {
		if err := copyFile(self, dest); err != nil {
			return fmt.Errorf("複製程式失敗：%w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "說明.txt"), []byte(readmeTXT()), 0644); err != nil {
		return err
	}
	if err := registerContextMenu(dest); err != nil {
		return fmt.Errorf("註冊右鍵選單失敗：%w", err)
	}
	if err := writeUninstallKey(dest); err != nil {
		return err
	}
	startDir := startMenuDir()
	_ = os.MkdirAll(startDir, 0755)
	_ = os.Remove(legacyStartMenuLnk())
	_ = createShortcut(startMenuLnk(), dest, dir, "墨轉 Markdown 轉檔")
	_ = createShortcutArgs(uninstallStartMenuLnk(), dest, "--uninstall", dir, "解除安裝墨轉")
	_ = createShortcut(sendToLnk("轉成 Word (.docx)"), dest, dir, "docx")
	_ = patchSendTo(sendToLnk("轉成 Word (.docx)"), dest, "--to docx --notify")
	_ = createShortcut(sendToLnk("轉成 PDF"), dest, dir, "pdf")
	_ = patchSendTo(sendToLnk("轉成 PDF"), dest, "--to pdf --notify")
	return nil
}

func doUninstall() error {
	_ = unregisterContextMenu()
	_ = deleteKey(`Software\Microsoft\Windows\CurrentVersion\Uninstall\MdInk`)
	_ = os.RemoveAll(startMenuDir())
	_ = os.Remove(legacyStartMenuLnk())
	_ = os.Remove(sendToLnk("轉成 Word (.docx)"))
	_ = os.Remove(sendToLnk("轉成 PDF"))
	dir := installDir()
	_ = os.Remove(filepath.Join(dir, "說明.txt"))
	_ = os.Remove(filepath.Join(dir, "mdink.log"))

	exe := installedExe()
	self, _ := os.Executable()
	if !sameFile(self, exe) {
		_ = os.Remove(exe)
		_ = os.RemoveAll(dir)
	}
	return nil
}

func finishUninstall() {
	exe := installedExe()
	if _, err := os.Stat(exe); err != nil {
		_ = os.RemoveAll(installDir())
		return
	}
	scheduleSelfDelete(exe, installDir())
}

func scheduleSelfDelete(exe, dir string) {
	ps := fmt.Sprintf(
		"$exe='%s';$dir='%s';Start-Sleep -Seconds 1;"+
			"for($i=0;$i -lt 20;$i++){ if(-not (Test-Path -LiteralPath $exe)){ break }; "+
			"Remove-Item -LiteralPath $exe -Force -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 400 }; "+
			"Remove-Item -LiteralPath $dir -Recurse -Force -ErrorAction SilentlyContinue",
		psQuote(exe), psQuote(dir),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200 | 0x08000000,
	}
	_ = cmd.Start()
}

func registerContextMenu(exe string) error {
	for _, ext := range []string{".md", ".markdown", ".mdown", ".mkd"} {
		if err := registerExt(ext, exe); err != nil {
			return err
		}
	}
	return nil
}

func unregisterContextMenu() error {
	for _, ext := range []string{".md", ".markdown", ".mdown", ".mkd"} {
		_ = deleteKey(`Software\Classes\SystemFileAssociations\` + ext + `\shell\MdInk`)
		_ = deleteKey(`Software\Classes\` + ext + `\shell\MdInk`)
	}
	return nil
}

func registerExt(ext, exe string) error {
	root := `Software\Classes\SystemFileAssociations\` + ext + `\shell\MdInk`
	k, _, err := registry.CreateKey(registry.CURRENT_USER, root, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()
	_ = k.SetStringValue("MUIVerb", "墨轉 Markdown")
	_ = k.SetStringValue("Icon", exe)
	_ = k.SetStringValue("Position", "Top")
	_ = k.SetStringValue("SubCommands", "")

	items := []struct {
		key, label, extra string
	}{
		{"docx", "轉成 Word 文件 (.docx)", "--to docx --notify"},
		{"pdf", "轉成 PDF 文件 (.pdf)", "--to pdf --notify"},
		{"both", "兩種格式都轉", "--to docx,pdf --notify"},
	}
	for _, it := range items {
		sk, _, err := registry.CreateKey(registry.CURRENT_USER, root+`\shell\`+it.key, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		_ = sk.SetStringValue("MUIVerb", it.label)
		_ = sk.SetStringValue("Icon", exe)
		sk.Close()
		ck, _, err := registry.CreateKey(registry.CURRENT_USER, root+`\shell\`+it.key+`\command`, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		cmd := `"` + exe + `" ` + it.extra + ` "%1"`
		_ = ck.SetStringValue("", cmd)
		ck.Close()
	}
	return nil
}

func writeUninstallKey(exe string) error {
	k, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\MdInk`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return err
	}
	defer k.Close()
	_ = k.SetStringValue("DisplayName", appName)
	_ = k.SetStringValue("DisplayVersion", appVersion)
	_ = k.SetStringValue("Publisher", "MdInk")
	_ = k.SetStringValue("DisplayIcon", exe)
	_ = k.SetStringValue("InstallLocation", installDir())
	_ = k.SetStringValue("UninstallString", `"`+exe+`" --uninstall`)
	_ = k.SetStringValue("QuietUninstallString", `"`+exe+`" --uninstall --quiet`)
	_ = k.SetDWordValue("NoModify", 1)
	_ = k.SetDWordValue("NoRepair", 1)
	_ = k.SetDWordValue("NoElevateOnModify", 1)
	if st, err := os.Stat(exe); err == nil {
		_ = k.SetDWordValue("EstimatedSize", uint32(st.Size()/1024))
	}
	return nil
}

func deleteKey(path string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return err
	}
	names, _ := k.ReadSubKeyNames(-1)
	k.Close()
	for _, n := range names {
		_ = deleteKey(path + `\` + n)
	}
	return registry.DeleteKey(registry.CURRENT_USER, path)
}

func startMenuDir() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`, "墨轉 MdInk")
}

func startMenuLnk() string {
	return filepath.Join(startMenuDir(), "墨轉 MdInk.lnk")
}

func uninstallStartMenuLnk() string {
	return filepath.Join(startMenuDir(), "解除安裝.lnk")
}

func legacyStartMenuLnk() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`, "墨轉 MdInk.lnk")
}

func sendToLnk(name string) string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\SendTo`, name+".lnk")
}

func createShortcut(lnk, target, workdir, desc string) error {
	ps := fmt.Sprintf(
		"$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%s');$s.TargetPath='%s';$s.WorkingDirectory='%s';$s.Description='%s';$s.IconLocation='%s,0';$s.Save()",
		psQuote(lnk), psQuote(target), psQuote(workdir), psQuote(desc), psQuote(target),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("建立捷徑失敗：%s %w", string(out), err)
	}
	return nil
}

func createShortcutArgs(lnk, target, extraArgs, workdir, desc string) error {
	ps := fmt.Sprintf(
		"$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%s');$s.TargetPath='%s';$s.Arguments='%s';$s.WorkingDirectory='%s';$s.Description='%s';$s.IconLocation='%s,0';$s.Save()",
		psQuote(lnk), psQuote(target), psQuote(extraArgs), psQuote(workdir), psQuote(desc), psQuote(target),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("建立捷徑失敗：%s %w", string(out), err)
	}
	return nil
}

func patchSendTo(lnk, target, extraArgs string) error {
	return createShortcutArgs(lnk, target, extraArgs, filepath.Dir(target), extraArgs)
}

func psQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(dst)
	return os.Rename(tmp, dst)
}

func sameFile(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func readmeTXT() string {
	return helpText() + `

命令列：
  mdink.exe --to docx 檔案.md
  mdink.exe --to pdf 檔案.md
  mdink.exe --to docx,pdf 檔案.md
  mdink.exe --install
  mdink.exe --uninstall
`
}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
