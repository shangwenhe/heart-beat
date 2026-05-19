#!/bin/bash
# 心跳客户端 - 每30秒向服务端发送一次心跳
# Usage: 修改 SERVER_URL 为你的腾讯云服务器地址

SERVER_URL="http://127.0.0.1:8080/api/heartbeat"
INTERVAL=30

echo "heart-beat client started, target: $SERVER_URL"

while true; do
    if curl -s -m 5 -X POST "$SERVER_URL"; then
        echo "$(date '+%Y-%m-%d %H:%M:%S') heartbeat ok"
    else
        echo "$(date '+%Y-%m-%d %H:%M:%S') heartbeat FAILED (server unreachable or timeout)"
    fi
    sleep "$INTERVAL"
done
