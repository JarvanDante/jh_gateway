package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"jh_gateway/internal/util"
)

// addTraceToContext 添加OpenTelemetry trace context到gRPC上下文
func addTraceToContext(ctx context.Context, r *ghttp.Request) context.Context {
	// OpenTelemetry会自动处理trace context的传播
	// 我们不需要手动传递trace_id，只需要确保使用正确的context

	// 记录当前的TraceID（仅用于调试）
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		util.LogWithTrace(ctx, "debug", "使用OpenTelemetry TraceID: %s", traceID)
	}

	// 直接返回context，让OpenTelemetry自动处理trace传播
	return ctx
}

// addUserInfoToGRPCContext 将用户信息添加到 gRPC metadata 中
func addUserInfoToGRPCContext(ctx context.Context, r *ghttp.Request) context.Context {
	md := metadata.New(map[string]string{})

	// 添加管理员ID
	if adminIdVar := r.GetCtxVar("admin_id"); adminIdVar != nil {
		if id := adminIdVar.Int(); id > 0 {
			md.Set("admin_id", fmt.Sprint(id))
			util.LogWithTrace(ctx, "debug", "添加 admin_id 到 gRPC metadata: %d", id)
		}
	}

	// 安全修复(A4)：透传管理员 token 里的权威站点ID，供下游后台接口做租户校验。
	// 仅来自认证中间件（token），不接受请求头伪造。
	if adminSiteVar := r.GetCtxVar("admin_site_id"); adminSiteVar != nil {
		md.Set("admin_site_id", fmt.Sprint(adminSiteVar.Int()))
	}

	// 添加用户ID
	if userIdVar := r.GetCtxVar("user_id"); userIdVar != nil {
		if id := userIdVar.Int(); id > 0 {
			md.Set("user_id", fmt.Sprint(id))
			util.LogWithTrace(ctx, "debug", "添加 user_id 到 gRPC metadata: %d", id)
		}
	}

	// 添加用户名
	if usernameVar := r.GetCtxVar("username"); usernameVar != nil {
		if username := usernameVar.String(); username != "" {
			md.Set("username", username)
			util.LogWithTrace(ctx, "debug", "添加 username 到 gRPC metadata: %s", username)
		}
	}

	// 添加站点ID（优先域名解析上下文，兜底读取请求头）
	if siteIdVar := r.GetCtxVar("site_id"); siteIdVar != nil {
		if id := siteIdVar.Int64(); id > 0 {
			md.Set("site_id", fmt.Sprint(id))
			util.LogWithTrace(ctx, "debug", "添加 site_id 到 gRPC metadata(ctx): %d", id)
		}
	}
	if _, ok := md["site_id"]; !ok {
		if siteID := strings.TrimSpace(r.Header.Get("X-Site-Id")); siteID != "" {
			md.Set("site_id", siteID)
			util.LogWithTrace(ctx, "debug", "添加 site_id 到 gRPC metadata(header): %s", siteID)
		}
	}

	// 添加站点代码（优先域名解析上下文，兜底读取请求头）
	if siteCodeVar := r.GetCtxVar("site_code"); siteCodeVar != nil {
		if code := siteCodeVar.String(); code != "" {
			md.Set("site_code", code)
			util.LogWithTrace(ctx, "debug", "添加 site_code 到 gRPC metadata(ctx): %s", code)
		}
	}
	if _, ok := md["site_code"]; !ok {
		if siteCode := strings.TrimSpace(r.Header.Get("X-Site-Code")); siteCode != "" {
			md.Set("site_code", siteCode)
			util.LogWithTrace(ctx, "debug", "添加 site_code 到 gRPC metadata(header): %s", siteCode)
		}
	}

	// 添加货币（从域名解析中间件获取）
	if currencyVar := r.GetCtxVar("currency"); currencyVar != nil {
		if currency := currencyVar.String(); currency != "" {
			md.Set("currency", currency)
			util.LogWithTrace(ctx, "debug", "添加 currency 到 gRPC metadata: %s", currency)
		}
	}

	// 添加域名（从域名解析中间件获取）
	if domainVar := r.GetCtxVar("domain"); domainVar != nil {
		if domain := domainVar.String(); domain != "" {
			md.Set("domain", domain)
			util.LogWithTrace(ctx, "debug", "添加 domain 到 gRPC metadata: %s", domain)
		}
	}

	// 添加客户端IP
	if clientIP := r.GetClientIp(); clientIP != "" {
		md.Set("client_ip", clientIP)
		util.LogWithTrace(ctx, "debug", "添加 client_ip 到 gRPC metadata: %s", clientIP)
	}

	// 透传前端请求上下文，供下游记录登录日志
	if ua := r.Header.Get("User-Agent"); ua != "" {
		md.Set("user-agent", ua)
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		md.Set("referer", referer)
	} else if origin := r.Header.Get("Origin"); origin != "" {
		md.Set("referer", origin)
	}
	if path := r.URL.String(); path != "" {
		md.Set("path", path)
	}
	if deviceOS := r.Header.Get("X-Device-OS"); deviceOS != "" {
		md.Set("x-device-os", deviceOS)
	}
	if deviceBrowser := r.Header.Get("X-Device-Browser"); deviceBrowser != "" {
		md.Set("x-device-browser", deviceBrowser)
	}
	if network := r.Header.Get("X-Network-Type"); network != "" {
		md.Set("x-network-type", network)
	}
	if screen := r.Header.Get("X-Screen-Resolution"); screen != "" {
		md.Set("x-screen-resolution", screen)
	}

	// 登录网址只记录域名（带协议）
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = r.URL.Scheme
	}
	if scheme == "" {
		scheme = "https"
	}
	if host := r.Host; host != "" {
		md.Set("login-domain", scheme+"://"+host)
	}

	// 如果有metadata，则创建新的outgoing context
	if len(md) > 0 {
		return metadata.NewOutgoingContext(ctx, md)
	}

	return ctx
}

// callBackendGRPCMethod 根据 HTTP 路径调用对应的 《后台》 gRPC 方法
func callBackendGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文 - 只需要在这里添加一次
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return backendRoutes.Handle(ctx, conn, r)
}

// callFrontendGRPCMethod 根据 HTTP 路径调用对应的 《前台》 gRPC 方法
func callFrontendGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 前台接口需要添加域名解析信息到gRPC metadata
	// 域名解析中间件已经将site_id和site_code添加到context中
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling frontend gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return frontendRoutes.Handle(ctx, conn, r)
}

// callBalanceGRPCMethod 根据 HTTP 路径调用对应的 《Balance》 gRPC 方法
func callBalanceGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling Balance gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return balanceRoutes.Handle(ctx, conn, r)
}

// callGameGRPCMethod 根据 HTTP 路径调用对应的 《Game》 gRPC 方法
func callGameGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling Game gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return gameRoutes.Handle(ctx, conn, r)
}

// callManageGRPCMethod 根据 HTTP 路径调用对应的 《Manage》 gRPC 方法
func callManageGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling Manage gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return manageRoutes.Handle(ctx, conn, r)
}
