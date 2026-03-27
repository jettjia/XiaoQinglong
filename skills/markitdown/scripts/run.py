import base64
import json
import os
import pathlib
import subprocess
import sys


def main():
    raw = os.environ.get("SKILL_INPUT", "").strip()
    if not raw:
        print("SKILL_INPUT 为空")
        sys.exit(2)

    try:
        obj = json.loads(raw)
    except Exception:
        obj = {"content_base64": raw, "file_name": "input.bin"}

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

    def _import_markitdown():
        from markitdown import MarkItDown
        return MarkItDown

    try:
        MarkItDown = _import_markitdown()
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
            MarkItDown = _import_markitdown()
        except Exception as ee:
            print(f"markitdown import failed: {ee}")
            sys.exit(3)

    md = MarkItDown(enable_plugins=False)
    result = md.convert(dst)
    text = getattr(result, "text_content", "") or ""
    print(text.strip())


if __name__ == "__main__":
    main()
