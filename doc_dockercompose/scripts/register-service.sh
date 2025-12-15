#!/bin/bash

# 注册服务到 Consul 的脚本
# 使用方法: ./scripts/register-service.sh <service-name> <service-port> <consul-addr>

SERVICE_NAME=${1:-user-service}
SERVICE_PORT=${2:-8001}
CONSUL_ADDR=${3:-localhost:8500}

# 获取本机 IP（Docker 环境下使用容器名）
SERVICE_HOST=${SERVICE_NAME}

echo "Registering service: $SERVICE_NAME at $SERVICE_HOST:$SERVICE_PORT to Consul ($CONSUL_ADDR)"

curl -X PUT http://${CONSUL_ADDR}/v1/agent/service/register -d @- <<EOF
{
  "ID": "${SERVICE_NAME}-1",
  "Name": "${SERVICE_NAME}",
  "Address": "${SERVICE_HOST}",
  "Port": ${SERVICE_PORT},
  "Check": {
    "HTTP": "http://${SERVICE_HOST}:${SERVICE_PORT}/health",
    "Interval": "10s",
    "Timeout": "5s"
  }
}
EOF

echo ""
echo "Service registered successfully!"
