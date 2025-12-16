# JH Gateway - API 网关

基于 GoFrame 框架的微服务 API 网关，支持 HTTP 和 gRPC 协议转发。

## 功能特性

- **HTTP 转发** - 将 HTTP 请求转发到后端微服务
- **gRPC 转发** - 支持 gRPC 协议的反向代理
- **服务发现** - 集成 Consul 进行服务注册与发现
- **中间件支持** - 日志、限流、熔断、JWT 认证等
- **统一错误处理** - 标准化的错误响应格式

## 项目结构

```
jh_gateway/
├── config.yaml              # 配置文件
├── main.go                  # 入口文件
├── internal/
│   ├── middleware/          # 中间件
│   │   ├── auth_jwt.go     # JWT 认证
│   │   ├── circuit_breaker.go # 熔断器
│   │   ├── logging.go      # 日志记录
│   │   ├── rate_limit.go   # 限流
│   │   └── trace.go        # 链路追踪
│   ├── proxy/              # 代理转发
│   │   ├── proxy.go        # HTTP 转发
│   │   ├── grpc.go         # gRPC 连接管理
│   │   ├── gateway.go      # gRPC 网关
│   │   └── grpc_client.go  # gRPC 客户端调用
│   ├── registry/           # 服务注册
│   │   └── consul.go       # Consul 客户端
│   ├── router/             # 路由配置
│   │   └── router.go       # 路由注册
│   └── util/               # 工具函数
│       └── response.go     # 统一响应格式
└── docker-compose.yml      # Docker 编排文件
```

## 配置说明

### config.yaml

```yaml
server:
  address: ":8000" # HTTP 网关端口

grpcGateway:
  address: ":8003" # gRPC 网关端口

consul:
  addr: "172.19.0.29:8500" # Consul 地址

jwt:
  secret: "your-jwt-secret" # JWT 密钥

rateLimit:
  requestsPerMinute: 120 # 每分钟请求限制
```

## 部署运行

### 1. Docker 方式（推荐）

```bash
# 启动所有服务
docker-compose up --build

# 后台运行
docker-compose up -d --build

# 查看日志
docker-compose logs -f jh_gateway
```

### 2. 本地开发

```bash
# 安装依赖
go mod tidy

# 运行
go run main.go

# 调试模式
dlv debug --headless --listen=:2345 --api-version=2
```

## API 接口

### HTTP 接口

- **健康检查**: `GET /gateway/health`
- **用户服务**: `GET|POST|PUT|DELETE /api/user/*`
- **支付服务**: `GET|POST|PUT|DELETE /api/pay/*`
- **游戏服务**: `GET|POST|PUT|DELETE /api/game/*`

### gRPC 接口

gRPC 客户端连接到 `localhost:8003` 端口

## 中间件配置

### 启用/禁用中间件

在 `internal/router/router.go` 中配置：

```go
s.Group("/api/user", func(group *ghttp.RouterGroup) {
    group.Middleware(
        middleware.Logging,        // 日志记录
        middleware.Trace,          // 链路追踪
        middleware.RateLimit,      // 限流
        // middleware.JWTAuth,     // JWT 认证（可选）
        middleware.CircuitBreaker, // 熔断器
    )
    group.ALL("/*any", proxy.GRPCToHTTP("user-service"))
})
```

## 服务发现

网关通过 Consul 自动发现后端服务：

1. 后端服务启动时注册到 Consul
2. 网关从 Consul 查询服务地址
3. 动态转发请求到健康的服务实例

## 错误处理

统一的错误响应格式：

```json
{
  "code": 0,
  "msg": "success",
  "data": {...}
}
```

错误码说明：

- `0`: 成功
- `400`: 请求错误
- `401`: 未授权
- `429`: 请求过于频繁
- `500`: 服务器错误
- `503`: 服务不可用

## 监控与调试

### Consul UI

访问 http://localhost:8500 查看服务状态

### 日志查看

```bash
# Docker 方式
docker-compose logs -f jh_gateway

# 本地方式
查看控制台输出
```

### 性能监控

- 请求日志包含响应时间
- 限流状态通过 HTTP 头返回
- 熔断器状态记录在日志中

## 开发指南

### 添加新的微服务

1. 在 `internal/router/router.go` 中添加路由组
2. 配置相应的中间件
3. 后端服务注册到 Consul

### 自定义中间件

1. 在 `internal/middleware/` 目录创建新文件
2. 实现 `ghttp.HandlerFunc` 接口
3. 在路由中注册使用

## 故障排查

### 常见问题

1. **服务不可用 (503)**

   - 检查后端服务是否启动
   - 确认 Consul 中服务状态为健康

2. **连接超时**

   - 检查网络连接
   - 确认服务地址和端口正确

3. **认证失败 (401)**
   - 检查 JWT token 是否有效
   - 确认 JWT 密钥配置正确
