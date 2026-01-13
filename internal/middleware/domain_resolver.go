package middleware

import (
	"context"
	"log"
	"strconv"
	"strings"

	"jh_gateway/internal/registry"

	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc/metadata"
)

// DomainResolver 域名解析中间件
func DomainResolver(r *ghttp.Request) {
	// 获取请求的Host头
	host := r.Header.Get("Host")
	if host == "" {
		host = r.Host
	}

	// 移除端口号（如果有）
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	log.Printf("请求域名: %s", host)

	// 根据域名获取site_id
	siteID, exists := registry.GetSiteIDByDomain(host)
	if exists {
		// 将site_id设置到请求上下文中
		r.SetCtxVar("site_id", siteID)
		log.Printf("域名 %s 解析到 site_id: %d", host, siteID)
	} else {
		// 如果没有找到对应的site_id，可以设置默认值或记录警告
		log.Printf("警告: 域名 %s 未找到对应的site_id，使用默认值", host)
		r.SetCtxVar("site_id", int64(0)) // 设置默认值
	}

	// 继续处理请求
	r.Middleware.Next()
}

// AddSiteIDToGRPCContext 将site_id添加到gRPC上下文中
func AddSiteIDToGRPCContext(ctx context.Context, r *ghttp.Request) context.Context {
	// 从请求上下文中获取site_id
	if siteIdVar := r.GetCtxVar("site_id"); siteIdVar != nil {
		if siteID := siteIdVar.Int64(); siteID > 0 {
			// 创建gRPC metadata
			md := metadata.New(map[string]string{
				"site_id": siteIdVar.String(),
			})

			// 将metadata添加到现有的outgoing context中
			if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
				// 合并现有的metadata
				for key, values := range existingMD {
					for _, value := range values {
						md.Append(key, value)
					}
				}
			}

			ctx = metadata.NewOutgoingContext(ctx, md)
			log.Printf("已将 site_id: %d 添加到 gRPC metadata", siteID)
		}
	}

	return ctx
}

// GetSiteIDFromRequest 从请求中获取site_id
func GetSiteIDFromRequest(r *ghttp.Request) int64 {
	if siteIdVar := r.GetCtxVar("site_id"); siteIdVar != nil {
		return siteIdVar.Int64()
	}
	return 0
}

// GetSiteIDFromContext 从上下文中获取site_id（用于gRPC服务端）
func GetSiteIDFromContext(ctx context.Context) int64 {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if siteIDs := md.Get("site_id"); len(siteIDs) > 0 {
			if siteID := strings.TrimSpace(siteIDs[0]); siteID != "" {
				// 转换为int64
				if id, err := strconv.ParseInt(siteID, 10, 64); err == nil {
					return id
				}
			}
		}
	}
	return 0
}
