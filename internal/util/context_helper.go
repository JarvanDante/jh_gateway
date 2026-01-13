package util

import (
	"context"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"
)

// GetSiteIDFromGRPCContext 从gRPC上下文中获取site_id
// 这个函数可以被其他微服务使用
func GetSiteIDFromGRPCContext(ctx context.Context) int64 {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if siteIDs := md.Get("site_id"); len(siteIDs) > 0 {
			if siteID := strings.TrimSpace(siteIDs[0]); siteID != "" {
				if id, err := strconv.ParseInt(siteID, 10, 64); err == nil {
					return id
				}
			}
		}
	}
	return 0
}

// GetAdminIDFromGRPCContext 从gRPC上下文中获取admin_id
func GetAdminIDFromGRPCContext(ctx context.Context) int64 {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if adminIDs := md.Get("admin_id"); len(adminIDs) > 0 {
			if adminID := strings.TrimSpace(adminIDs[0]); adminID != "" {
				if id, err := strconv.ParseInt(adminID, 10, 64); err == nil {
					return id
				}
			}
		}
	}
	return 0
}

// GetUserIDFromGRPCContext 从gRPC上下文中获取user_id
func GetUserIDFromGRPCContext(ctx context.Context) int64 {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if userIDs := md.Get("user_id"); len(userIDs) > 0 {
			if userID := strings.TrimSpace(userIDs[0]); userID != "" {
				if id, err := strconv.ParseInt(userID, 10, 64); err == nil {
					return id
				}
			}
		}
	}
	return 0
}

// GetClientIPFromGRPCContext 从gRPC上下文中获取client_ip
func GetClientIPFromGRPCContext(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if clientIPs := md.Get("client_ip"); len(clientIPs) > 0 {
			return strings.TrimSpace(clientIPs[0])
		}
	}
	return ""
}
