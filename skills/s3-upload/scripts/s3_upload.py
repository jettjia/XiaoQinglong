#!/usr/bin/env python3
import os
import sys
import json
import argparse

try:
    import boto3
    from botocore.config import Config
    from botocore.exceptions import ClientError
except ImportError:
    print(json.dumps({
        "error": "boto3 is not installed. Install with: pip3 install boto3"
    }))
    sys.exit(1)


def get_content_type(key):
    ext = os.path.splitext(key)[1].lower()
    types = {
        ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
        ".pdf": "application/pdf",
        ".png": "image/png",
        ".jpg": "image/jpeg",
        ".jpeg": "image/jpeg",
        ".gif": "image/gif",
        ".txt": "text/plain",
        ".html": "text/html",
        ".css": "text/css",
        ".js": "application/javascript",
        ".json": "application/json",
        ".xml": "application/xml",
        ".zip": "application/zip",
        ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    }
    return types.get(ext, "application/octet-stream")


def get_config_from_env():
    return {
        "endpoint": os.environ.get("S3_ENDPOINT", ""),
        "access_key": os.environ.get("S3_ACCESS_KEY", ""),
        "secret_key": os.environ.get("S3_SECRET_KEY", ""),
        "bucket": os.environ.get("S3_BUCKET", ""),
        "region": os.environ.get("S3_REGION", "us-east-1"),
        "prefix": os.environ.get("S3_PREFIX", ""),
    }


def parse_input():
    input_str = os.environ.get("SKILL_INPUT", "").strip()
    if not input_str:
        return {}
    try:
        return json.loads(input_str)
    except json.JSONDecodeError:
        return {"raw_input": input_str}


def upload_to_s3(config, input_data):
    file_path = input_data.get("file_path", "").strip()
    object_key = input_data.get("object_key", "").strip()

    if not file_path:
        return {"error": "file_path is required"}

    if not object_key:
        object_key = os.path.basename(file_path)

    # 避免重复添加 prefix
    prefix = config.get("prefix", "").strip("/")
    if prefix and not object_key.startswith(prefix + "/"):
        object_key = f"{prefix}/{object_key}"

    # 支持绝对路径和相对路径
    search_paths = []
    if os.path.isabs(file_path):
        search_paths.append(file_path)
    else:
        search_paths.extend([
            file_path,
            f"/workspace/{file_path}",
            f"/workspace/{input_data.get('raw_input', '')}",
            f"/skills/{file_path}",
            f"/skills/output/{file_path}",
            f"/skills/s3-upload/{file_path}",
            f"output/{file_path}",
        ])
    actual_path = None
    for p in search_paths:
        # 跳过空路径和目录
        if p and p != "/workspace/" and os.path.isfile(p):
            actual_path = p
            break

    if not actual_path:
        return {"error": f"File not found: {file_path}"}

    access_key = config.get("access_key", "")
    secret_key = config.get("secret_key", "")
    bucket = config.get("bucket", "")
    region = config.get("region", "us-east-1")
    endpoint = config.get("endpoint", "")

    client_kwargs = {"region_name": region}

    if endpoint and access_key and secret_key:
        client_kwargs.update({
            "endpoint_url": endpoint,
            "aws_access_key_id": access_key,
            "aws_secret_access_key": secret_key,
        })
        if "aliyun" in endpoint.lower() or "aliyuncs" in endpoint.lower():
            # 阿里云 OSS 使用 virtual hosted style
            client_kwargs["config"] = Config(
                signature_version="s3",
                s3={"addressing_style": "virtual"},
            )

    try:
        client = boto3.client("s3", **client_kwargs)
    except Exception as e:
        return {"error": f"Failed to create S3 client: {str(e)}"}

    if not bucket:
        return {"error": "S3_BUCKET is not configured"}

    try:
        file_size = os.path.getsize(actual_path)
        content_type = get_content_type(object_key)

        client.upload_file(
            actual_path,
            bucket,
            object_key,
            ExtraArgs={"ContentType": content_type},
        )

        if endpoint:
            endpoint_clean = endpoint.rstrip("/").replace("https://", "").replace("http://", "")
            # 阿里云 OSS 使用 virtual hosted style: https://bucket.endpoint/key
            url = f"https://{bucket}.{endpoint_clean}/{object_key}"
        else:
            url = f"https://{bucket}.s3.{region}.amazonaws.com/{object_key}"

        return {
            "url": url,
            "object_key": object_key,
            "bucket": bucket,
            "size": file_size,
            "content_type": content_type,
        }

    except ClientError as e:
        return {"error": f"Upload failed: {str(e)}"}
    except Exception as e:
        return {"error": f"Unexpected error: {str(e)}"}


def main():
    input_data = parse_input()
    config = get_config_from_env()

    result = upload_to_s3(config, input_data)

    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
