# 域名映射功能说明

## 功能概述

Gateway 现在支持从 Consul KV 中监听域名配置，并自动构建域名到 site_id 的映射关系。当请求到达时，会根据请求的 Host 头自动解析出对应的 site_id，并通过 gRPC metadata 传递给下游微服务。

## 架构设计

```
请求 -> Gateway -> 域名解析中间件 -> gRPC调用 -> 微服务
                      ↓
                 Consul KV监听器
                      ↓
                 域名映射缓存
```

## 核心组件

### 1. 域名监听器 (DomainWatcher)

- 文件：`internal/registry/domain_watcher.go`
- 功能：监听 Consul KV 中的域名配置变化
- KV 路径格式：`sites/{site_code}/domains`
- 数据格式：

```json
{
  "site_id": 1,
  "site_code": "by",
  "domains": {
    "API": ["xbdb@qq.com", "a2222bc.com"],
    "H5": ["5.dj1fhuxit@qq.com", "xit@qq.com", "xzy.com"]
  },
  "total_count": 5,
  "published_at": "2006-01-02 15:04:05",
  "version": 1768300409
}
```

### 2. 域名解析中间件 (DomainResolver)

- 文件：`internal/middleware/domain_resolver.go`
- 功能：从 HTTP 请求中提取域名，查找对应的 site_id
- 将 site_id 设置到请求上下文中

### 3. gRPC 上下文传递

- 修改了`addUserInfoToGRPCContext`函数
- 自动将 site_id 添加到 gRPC metadata 中
- 下游微服务可以通过 metadata 获取 site_id

## 使用方法

### Gateway 端配置

1. **启动时自动初始化**

   - Gateway 启动时会自动初始化 Consul 客户端和域名监听器
   - 无需额外配置

2. **中间件配置**
   - 在需要域名解析的路由组中添加`middleware.DomainResolver`
   - 目前已在`/api/frontend`路由组中启用

### 微服务端使用

在微服务中获取 site_id：

```go
import (
    "context"
    "google.golang.org/grpc/metadata"
)

func (s *YourService) YourMethod(ctx context.Context, req *YourRequest) (*YourResponse, error) {
    // 方法1：直接从metadata获取
    if md, ok := metadata.FromIncomingContext(ctx); ok {
        if siteIDs := md.Get("site_id"); len(siteIDs) > 0 {
            siteID, _ := strconv.ParseInt(siteIDs[0], 10, 64)
            // 使用siteID
        }
    }

    // 方法2：使用工具函数（推荐）
    siteID := util.GetSiteIDFromGRPCContext(ctx)
    if siteID > 0 {
        // 使用siteID进行业务逻辑
    }

    return &YourResponse{}, nil
}
```

## 调试接口

### 查看域名映射状态

```bash
GET /gateway/domain-mappings
```

响应示例：

```json
{
  "code": 0,
  "msg": "success",
  "data": {
    "mappings": {
      "xbdb@qq.com": 1,
      "a2222bc.com": 1,
      "5.dj1fhuxit@qq.com": 1,
      "xit@qq.com": 1,
      "xzy.com": 1
    },
    "stats": {
      "total_domains": 5,
      "total_sites": 1,
      "status": "running"
    }
  }
}
```

## 配置发布流程

1. **在管理后台配置域名**

   - 通过域名管理页面添加/编辑域名
   - 设置域名类型（API、H5、Admin、Callback）

2. **发布域名配置**

   - 点击"发布"按钮
   - 系统会将域名配置发布到 Consul KV

3. **自动同步**
   - Gateway 的域名监听器会自动检测到配置变化
   - 更新内存中的域名映射缓存

## 注意事项

1. **域名格式**

   - 支持标准域名格式
   - 会自动去除端口号
   - 区分大小写

2. **容错处理**

   - 如果域名未找到对应的 site_id，会设置默认值 0
   - 监听器会自动重连和重试

3. **性能考虑**

   - 域名映射存储在内存中，查询性能很高
   - 使用读写锁保证并发安全
   - 长轮询机制减少不必要的网络请求

4. **监控和日志**
   - 所有域名解析过程都有详细日志
   - 可以通过调试接口查看当前状态

## 扩展功能

未来可以考虑添加：

- 域名配置的版本管理
- 域名解析的性能监控
- 域名配置的热更新通知
- 多数据中心的域名同步
