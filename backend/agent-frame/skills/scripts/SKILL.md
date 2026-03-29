---
name: s3-upload-3
description: Upload generated files to S3-compatible storage. Use this skill when user wants to save, download, or share files (like PPTX, PDF, images) to cloud storage. This skill uploads files to S3 and returns a shareable URL.
output_patterns:
  - "*.pptx"
  - "*.pdf"
  - "*.png"
  - "*.jpg"
---

# S3 Upload Skill

## Quick Reference

| Task | Guide |
|------|-------|
| Upload to S3 | Run `python3 /workspace/s3-upload/scripts/s3_upload.py` |

## Usage

This skill requires environment variables or input parameters for S3 configuration.

### Required Parameters (via input)

- `file_path`: Path to the file to upload (e.g., "kubernetes.pptx")
- `object_key`: The S3 object key (e.g., "uploads/presentation.pptx")

### Configuration (via environment variables)

- `S3_ENDPOINT`: S3 endpoint URL (e.g., https://oss-cn-shanghai.aliyuncs.com)
- `S3_ACCESS_KEY`: Access key ID
- `S3_SECRET_KEY`: Secret access key
- `S3_BUCKET`: Bucket name
- `S3_REGION`: Region (default: us-east-1)
- `S3_PREFIX`: Optional prefix for all uploads

### Input Format

```json
{
  "file_path": "kubernetes.pptx",
  "object_key": "uploads/k8s-presentation.pptx"
}
```

### Output

```json
{
  "url": "https://xdata-test.oss-cn-shanghai.aliyuncs.com/uploads/k8s-presentation.pptx",
  "object_key": "uploads/k8s-presentation.pptx",
  "size": 62630
}
```

## Environment Variable Priority

1. First: Check environment variables
2. Fallback: Check sandbox config (sandbox.env)
