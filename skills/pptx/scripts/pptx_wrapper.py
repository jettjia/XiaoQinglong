#!/usr/bin/env python3
"""
PPTX Smart Wrapper - 简化版 PPT 创建工具
接受 JSON 配置，自动创建专业 PPT
"""

import json
import os
import sys
import subprocess
import tempfile
from pathlib import Path

def run_command(cmd, cwd=None):
    result = subprocess.run(cmd, shell=True, capture_output=True, text=True, cwd=cwd)
    return result.stdout, result.stderr, result.returncode

def create_pptx_from_config(config):
    """
    根据配置创建 PPTX
    config: JSON 字符串，包含 slides 列表
    """
    slides = config.get("slides", [])
    if not slides:
        return {"error": "No slides provided. Usage: python3 /skills/scripts/pptx_wrapper.py '{\"slides\":[{\"title\":\"K8s\",\"content\":\"内容\"}]}'"}

    output_dir = "/skills/output"
    os.makedirs(output_dir, exist_ok=True)

    output_file = os.path.join(output_dir, config.get("filename", "presentation.pptx"))

    # 构建 pptxgenjs 脚本
    js_script = f"""
const pptxgen = require("pptxgenjs");
const pres = new pptxgen();
pres.layout = "LAYOUT_16x9";
pres.title = "{config.get("title", "Presentation")}";
pres.author = "{config.get("author", "AI Assistant")}";

{chr(10).join([f'''
const slide{i} = pres.addSlide();
slide{i}.background = {{ color: "{s.get("bg", "FFFFFF")}" }};
slide{i}.addText("{s.get("title", "")}", {{
    x: 0.5, y: 0.5, w: 9, h: 1,
    fontSize: {s.get("title_size", 36)},
    color: "{s.get("title_color", "1E2761")}",
    bold: true
}});
slide{i}.addText(`{s.get("content", "")}`, {{
    x: 0.5, y: 1.8, w: 9, h: 3.5,
    fontSize: {s.get("content_size", 18)},
    color: "{s.get("content_color", "363636")}",
    valign: "top"
}});
''' for i, s in enumerate(slides)])}

pres.writeFile({{ fileName: "{output_file}" }})
.then(() => console.log("Created:", "{output_file}"))
.catch(err => console.error("Error:", err));
"""

    # 写入临时 JS 文件
    tmp_js = "/tmp/pptx_gen.js"
    with open(tmp_js, "w") as f:
        f.write(js_script)

    # 执行
    stdout, stderr, code = run_command(f"cd /tmp && npm install pptxgenjs --silent 2>/dev/null")
    stdout, stderr, code = run_command(f"node {tmp_js}")

    if code != 0:
        return {"error": f"Failed to create PPTX: {stderr}"}

    return {
        "success": True,
        "output": output_file,
        "slides_count": len(slides)
    }

def main():
    if len(sys.argv) < 2:
        print(json.dumps({
            "usage": "python3 /skills/scripts/pptx_wrapper.py '{\"title\":\"K8s\",\"slides\":[{\"title\":\"K8s简介\",\"content\":\"Kubernetes是...\"},{\"title\":\"架构\",\"content\":\"Master+Node架构\"}]}'",
            "example": {
                "title": "K8s Presentation",
                "author": "AI",
                "filename": "k8s.pptx",
                "slides": [
                    {"title": "Kubernetes简介", "bg": "F5F5F5", "title_color": "1E2761", "content": "Kubernetes是容器编排平台..."},
                    {"title": "核心架构", "bg": "1E2761", "title_color": "FFFFFF", "content_color": "FFFFFF", "content": "Master节点 + Worker节点"}
                ]
            }
        }, indent=2, ensure_ascii=False))
        return

    try:
        config = json.loads(sys.argv[1])
    except json.JSONDecodeError as e:
        print(json.dumps({"error": f"Invalid JSON: {e}"}))
        return

    result = create_pptx_from_config(config)
    print(json.dumps(result, indent=2, ensure_ascii=False))

if __name__ == "__main__":
    main()
