# Docker Compose 环境搭建指南

## 快速开始

### 1. 启动 Consul 和网关

```bash
docker-compose up -d
```

这会启动：

- Consul 服务器（端口 8500）
- jh-gateway 网关（端口 8901）

### 2. 验证 Consul 运行

访问 Consul UI：http://localhost:8500/ui

### 3. 注册微服务

如果你的微服务已经在运行，可以手动注册到 Consul：

```bash
# 注册 user-service
curl -X PUT http://localhost:8500/v1/agent/service/register -d '{
  "ID": "user-service-1",
  "Name": "user-service",
  "Address": "host.docker.internal",
  "Port": 8001,
  "Check": {
    "HTTP": "http://host.docker.internal:8001/health",
    "Interval": "10s",
    "Timeout": "5s"
  }
}'

# 注册 payment-service
curl -X PUT http://localhost:8500/v1/agent/service/register -d '{
  "ID": "payment-service-1",
  "Name": "payment-service",
  "Address": "host.docker.internal",
  "Port": 8002,
  "Check": {
    "HTTP": "http://host.docker.internal:8002/health",
    "Interval": "10s",
    "Timeout": "5s"
  }
}'

# 注册 game-service
curl -X PUT http://localhost:8500/v1/agent/service/register -d '{
  "ID": "game-service-1",
  "Name": "game-service",
  "Address": "host.docker.internal",
  "Port": 8003,
  "Check": {
    "HTTP": "http://host.docker.internal:8003/health",
    "Interval": "10s",
    "Timeout": "5s"
  }
}'
```

### 4. 测试网关

```bash
# 测试健康检查
curl http://localhost:8901/gateway/health

# 测试转发（假设 user-service 已注册）
curl http://localhost:8901/api/user/list
```

## 常用命令

```bash
# 查看所有服务
curl http://localhost:8500/v1/catalog/services

# 查看特定服务的实例
curl http://localhost:8500/v1/catalog/service/user-service

# 停止所有容器
docker-compose down

# 查看日志
docker-compose logs -f jh-gateway
docker-compose logs -f consul
```

## 网络说明

- 所有容器都在 `jh-network` 网络中
- 容器间通过容器名通信（如 `consul:8500`）
- 从宿主机访问使用 `localhost` 或 `host.docker.internal`

## 如果微服务在宿主机上运行

在 docker-compose.yml 中，使用 `host.docker.internal` 作为地址：

```yaml
services:
  user-service:
    image: your-user-service:latest
    ports:
      - "8001:8001"
    environment:
      - CONSUL_ADDR=host.docker.internal:8500
```

或者在注册服务时使用 `host.docker.internal`。
