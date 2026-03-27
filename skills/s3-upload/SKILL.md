---
name: s3-upload
description: Upload files to S3. The script reads SKILL_INPUT env var for file_path and object_key.
---

# S3 Upload Skill

## Quick Reference

| Task | Command |
|------|---------|
| Upload to S3 | `python3 /workspace/scripts/s3_upload.py` |

## How to Use

1. SKILL_INPUT 环境变量包含输入参数 (JSON格式: {"file_path": "xxx", "object_key": "xxx"})
2. 环境变量已配置: S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_BUCKET
3. 直接运行: `python3 /workspace/scripts/s3_upload.py`
