---
title: 墨轉測試文件
author: Andy
---

# 墨轉能處理什麼

這份文件用來確認 **粗體**、*斜體*、~~刪除線~~ 與 `行內程式碼` 都能正確進 Word。

## 清單

- 繁體中文標題與內文
- 表格、連結與引用
- [Markdown 指南](https://www.markdownguide.org)

1. 安裝後對 `.md` 按右鍵
2. 選「顯示更多選項」
3. 轉成 Word 或 PDF

## 程式碼

```go
package main

func main() {
    println("hello, 墨轉")
}
```

> 輸出檔會放在 Markdown 同一個資料夾。

## 表格

| 格式 | 用途 | 中文 |
| --- | --- | --- |
| DOCX | 再編輯 | 支援 |
| PDF | 分享列印 | 支援 |

任務：

- [x] 標題與段落
- [ ] 圖片（相對路徑）
