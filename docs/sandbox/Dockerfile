# 基础镜像：阿里opensandbox
FROM sandbox-registry.cn-zhangjiakou.cr.aliyuncs.com/opensandbox/code-interpreter:v1.0.2

# 安装 Node.js 和 npm（如果没有的话）
RUN apt-get update && apt-get install -y \
    nodejs \
    npm \
    python3-pip \
    && pip3 install --no-cache-dir --break-system-packages \
        Pillow \
        defusedxml \
        lxml \
        markitdown \
        numpy \
        pandas \
        requests \
        boto3 \
        python-pptx \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# 预先安装 pptxgenjs 到全局（避免每次运行时重新安装）
RUN npm install -g pptxgenjs --silent

# 设置 NODE_PATH 让 require() 能找到全局模块
ENV NODE_PATH=/usr/local/lib/node_modules

# 保持原来的入口脚本
ENTRYPOINT ["/opt/opensandbox/code-interpreter.sh"]
