---
name: markitdown
description: 使用 Microsoft MarkItDown 将文件转换为 Markdown 文本。支持文件路径和 base64 两种输入方式。
---

```bash
python3 {{.BaseDirectory}}/scripts/run.py
```

## 输入格式

### 方式1: 文件路径（推荐）
```json
{
  "file_path": "/mnt/uploads/session_xxx/report.pdf"
}
```

### 方式2: Base64
```json
{
  "content_base64": "base64编码的文件内容",
  "file_name": "report.pdf"
}
```

## 支持的文件类型

| 类型 | 处理方式 |
|------|----------|
| 文本文件 (.txt, .md, .csv, .json, .xml, .html, .py, .js 等) | 直接读取文本 |
| PDF (.pdf) | MarkItDown 转换为 Markdown |
| Word (.docx) | MarkItDown 转换为 Markdown |
| Excel (.xlsx) | MarkItDown 转换为 Markdown |
| PowerPoint (.pptx) | MarkItDown 转换为 Markdown |

