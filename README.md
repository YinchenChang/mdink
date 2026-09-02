# 墨轉 MdInk

Windows 11 右鍵把 Markdown（`.md`）轉成 Word（`.docx`）或 PDF。

## 安裝

1. 解壓縮後執行 `MdInk-Setup.exe`（不需系統管理員）
2. Chrome / SmartScreen 若提示「可能有害」，選「其他資訊」→「仍要執行」

安裝後：對 `.md` 檔按右鍵 → **顯示更多選項** → **墨轉 Markdown**。

Windows 11 精簡右鍵選單無法由一般程式直接加入項目，這是系統限制。可用 Shift + 右鍵，或開始功能表的「墨轉」捷徑。

## 解除安裝

- 設定 → 應用程式 → 已安裝的應用程式 → 墨轉 MdInk
- 開始功能表 → 墨轉 MdInk → 解除安裝
- 再執行安裝檔並選擇解除安裝

轉換過的 Word / PDF 不會被刪除。

## 從原始碼編譯

需要 Go 1.23+。

```bat
cd windows\mdink
set CGO_ENABLED=0
go build -ldflags "-s -w -H windowsgui" -o MdInk-Setup.exe .
```

程式會安裝到 `%LOCALAPPDATA%\Programs\MdInk`，右鍵選單寫在目前使用者的登錄檔。

## 授權

個人使用。未經簽署的安裝檔可能被 SmartScreen 提示，這是預期行為。
