# heart-beat Makefile
#
# 服务端（腾讯云服务器）:
#   make server
#
# 客户端（家里 Ubuntu）:
#   make client SERVER_URL=http://1.2.3.4:51502
#
# 卸载:
#   make server-remove
#   make client-remove

BINARY    := heart-beat
LISTEN    := :51502
DB_PATH   := /var/lib/heartbeat/heartbeat.db
SERVER_URL ?= http://127.0.0.1:51502/api/heartbeat

.PHONY: server server-remove client client-remove build clean logs status

# ========== 构建 ==========

build:
	GOPROXY=https://goproxy.cn,direct go build -o $(BINARY) .

clean:
	rm -f $(BINARY)

# ========== 服务端（腾讯云） ==========

server: build
	@echo ">>> 安装 heartbeat 服务端"
	install -d /var/lib/heartbeat
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
	install -m 644 deploy/heart-beat.service /etc/systemd/system/heart-beat.service
	systemctl daemon-reload
	systemctl enable heart-beat
	systemctl restart heart-beat
	@echo ""
	@echo ">>> 安装完成！"
	@echo "    访问: http://localhost:51502"
	@echo "    数据: $(DB_PATH)"
	@echo ""
	@echo "    微信告警（可选）:"
	@echo "    vim /etc/systemd/system/heart-beat.service"
	@echo "    取消注释 ALERT_PROVIDER 和 ALERT_WEBHOOK"
	@echo "    systemctl daemon-reload && systemctl restart heart-beat"

server-remove:
	systemctl stop heart-beat || true
	systemctl disable heart-beat || true
	rm -f /usr/local/bin/$(BINARY)
	rm -f /etc/systemd/system/heart-beat.service
	systemctl daemon-reload
	@echo ">>> 服务端已卸载（数据保留: $(DB_PATH)）"
	@echo "    如需删除数据: rm -rf /var/lib/heartbeat"

# ========== 客户端（家里 Ubuntu） ==========

client:
	@echo ">>> 安装 heartbeat 客户端"
	sed 's|http://127.0.0.1:51502/api/heartbeat|$(SERVER_URL)|' deploy/heartbeat-client.sh > /tmp/heartbeat-client.sh
	install -m 755 /tmp/heartbeat-client.sh /usr/local/bin/heartbeat-client.sh
	rm -f /tmp/heartbeat-client.sh
	install -m 644 deploy/heartbeat-client.service /etc/systemd/system/heartbeat-client.service
	systemctl daemon-reload
	systemctl enable heartbeat-client
	systemctl restart heartbeat-client
	@echo ""
	@echo ">>> 安装完成！"
	@echo "    目标: $(SERVER_URL)"

client-remove:
	systemctl stop heartbeat-client || true
	systemctl disable heartbeat-client || true
	rm -f /usr/local/bin/heartbeat-client.sh
	rm -f /etc/systemd/system/heartbeat-client.service
	systemctl daemon-reload
	@echo ">>> 客户端已卸载"

# ========== 日志 & 状态 ==========

logs:
	journalctl -u heart-beat -f

status:
	systemctl status heart-beat

client-logs:
	journalctl -u heartbeat-client -f

client-status:
	systemctl status heartbeat-client
