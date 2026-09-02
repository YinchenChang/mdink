# 墨轉 MdInk

Windows 11 右鍵把 Markdown（`.md`）轉成 Word（`.docx`）或 PDF。

- 不需系統管理員
- 安裝到目前使用者的 `%LOCALAPPDATA%\Programs\MdInk`
- 右鍵選單出現在「顯示更多選項」（或 Shift + 右鍵）
- 完整解除安裝：登錄檔、捷徑、程式檔都會清掉；已轉出的 Word / PDF 不會刪

## 從原始碼編譯

需要 Go 1.23+（Windows amd64）。

```bat
cd windows\mdink
set CGO_ENABLED=0
go build -ldflags "-s -w -H windowsgui" -o MdInk-Setup.exe .
```

執行產生的 `MdInk-Setup.exe` 即可安裝。

## 使用

安裝後：對 `.md` 檔按右鍵 → **顯示更多選項** → **墨轉 Markdown**。

Windows 11 精簡右鍵選單無法由一般 Win32 程式直接加入項目，這是系統限制。可用 Shift + 右鍵，或開始功能表的「墨轉」捷徑。

## 解除安裝

- 設定 → 應用程式 → 已安裝的應用程式 → 墨轉 MdInk
- 開始功能表 → 墨轉 MdInk → 解除安裝
- 再執行安裝檔並選擇解除安裝

## 安裝檔校驗

預先編譯的安裝檔不放在這個 repo（`.exe` / `.zip` 已列入 `.gitignore`，避免 GitHub 擋未簽署執行檔）。

若你手上有安裝檔，SHA-256 應為：

| 檔案 | SHA-256 |
| --- | --- |
| MdInk-Setup.exe | 620376bc7b7dc00d6efe4d58f88526a7dfa16c1c62c3392cb637bbfd35d2b414 |
| MdInk-Setup.zip | 4d7128189c01701a4135324583f4c72af456480907eacc80471721260cc02efb |

詳見 [`dist/說明.txt`](dist/說明.txt)。

## 授權

個人使用。未經簽署的安裝檔可能被 SmartScreen 提示，這是預期行為。
