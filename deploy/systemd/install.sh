#!/bin/bash
set -e

SERVICE=kratos  
BIN_SRC="./kratos"
BIN_DST="/usr/local/bin/kratos"
SERVICE_FILE="kratos.service"
SERVICE_DST="/etc/systemd/system/$SERVICE.service"
CONFIG_SRC="./configs/config.yaml"
CONFIG_DST="/etc/kratos/config.yaml"

mkdir -p /etc/kratos
mkdir -p /usr/lib/kratos

# 复制二进制
if [ -f "$BIN_SRC" ]; then
    install -Dm755 "$BIN_SRC" "$BIN_DST"
else
    echo "二进制文件 $BIN_SRC 未找到，请先编译或调整路径。"
    exit 1
fi

# 复制配置文件
if [ -f "$CONFIG_SRC" ]; then
    install -Dm644 "$CONFIG_SRC" "$CONFIG_DST"
else
    echo "配置文件 $CONFIG_SRC 未找到，请先准备配置文件。"
    exit 1
fi

# 复制 systemd 服务文件
install -Dm644 "$SERVICE_FILE" "$SERVICE_DST"

# 重新加载 systemd
systemctl daemon-reload
# 启用并启动服务
systemctl enable $SERVICE
systemctl restart $SERVICE

echo "kratos 已安装并启动。" 