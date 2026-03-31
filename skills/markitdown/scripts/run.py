import base64
import json
import os
import pathlib
import subprocess
import sys


def read_file_by_path(file_path: str) -> str:
    """根据文件路径读取文件内容

    支持的文件类型：
    - 文本文件 (.txt, .md, .csv, .json, .xml, .html, .py 等): 直接读取
    - 二进制文件 (.pdf, .docx, .xlsx 等): 使用 markitdown 转换
    """
    path = pathlib.Path(file_path)
    if not path.exists():
        print(f"文件不存在: {file_path}")
        sys.exit(1)

    suffix = path.suffix.lower()

    # 文本文件直接读取
    text_suffixes = {".md", ".markdown", ".txt", ".csv", ".json", ".xml", ".html", ".htm",
                     ".py", ".js", ".ts", ".go", ".java", ".c", ".cpp", ".h", ".sh",
                     ".yaml", ".yml", ".toml", ".ini", ".conf", ".log"}
    if suffix in text_suffixes:
        try:
            return path.read_text(encoding="utf-8").strip()
        except Exception:
            return path.read_text(encoding="gbk").strip()

    # 二进制文件使用 markitdown 转换
    return convert_with_markitdown(str(path))


def convert_with_markitdown(file_path: str) -> str:
    """使用 markitdown 转换文件为 markdown"""
    try:
        from markitdown import MarkItDown
    except Exception as e:
        auto_install = str(os.environ.get("MARKITDOWN_AUTO_INSTALL", "1")).strip().lower() not in {"0", "false", "off", "no"}
        if not auto_install:
            print(f"markitdown import failed: {e}")
            sys.exit(3)

        timeout_s = 120
        try:
            timeout_s = int(str(os.environ.get("MARKITDOWN_INSTALL_TIMEOUT_S", "120")).strip())
        except Exception:
            timeout_s = 120

        if timeout_s <= 0:
            timeout_s = 120

        cmd = [sys.executable, "-m", "pip", "install", "-q", "--no-cache-dir", "markitdown[all]"]
        env = dict(os.environ)
        env["PIP_DISABLE_PIP_VERSION_CHECK"] = "1"
        env["PYTHONUNBUFFERED"] = "1"
        try:
            subprocess.run(cmd, env=env, check=False, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout_s)
        except Exception as ee:
            print(f"markitdown install failed: {ee}")
            sys.exit(3)

        try:
            from markitdown import MarkItDown
        except Exception as ee:
            print(f"markitdown import failed: {ee}")
            sys.exit(3)

    md = MarkItDown(enable_plugins=False)
    result = md.convert(file_path)
    return (getattr(result, "text_content", "") or "").strip()


def main():
    raw = os.environ.get("SKILL_INPUT", "").strip()
    if not raw:
        print("SKILL_INPUT 为空")
        sys.exit(2)

    try:
        obj = json.loads(raw)
    except Exception:
        obj = {"content_base64": raw, "file_name": "input.bin"}

    # 支持 file_path 参数（文件路径）
    file_path = str(obj.get("file_path") or "").strip()
    if file_path:
        # 直接读取文件
        text = read_file_by_path(file_path)
        print(text)
        return

    # 原有 base64 方式
    b64 = str(obj.get("content_base64") or "").strip()
    if not b64:
        print("缺少 content_base64")
        sys.exit(2)

    name = str(obj.get("file_name") or "input.bin").strip()
    name = pathlib.Path(name).name or "input.bin"
    mime = str(obj.get("mime") or "").strip().lower()
    dst = f"/workspace/{name}"

    try:
        data = base64.b64decode(b64)
    except Exception as e:
        print(f"base64 decode failed: {e}")
        sys.exit(2)

    suffix = pathlib.Path(name).suffix.lower()
    if mime.startswith("text/") or suffix in {".md", ".markdown", ".txt", ".csv", ".json", ".xml", ".html", ".htm"}:
        try:
            text = data.decode("utf-8", errors="replace").strip()
        except Exception:
            text = ""
        if text:
            print(text)
            return

    with open(dst, "wb") as f:
        f.write(data)

    # 使用 markitdown 转换
    text = convert_with_markitdown(dst)
    print(text)


if __name__ == "__main__":
    main()
