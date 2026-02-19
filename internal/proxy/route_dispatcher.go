package proxy

import (
	"context"
	"fmt"
	"strings"

	"jh_gateway/internal/middleware"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/tracing"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// ServiceRoute 服务路由配置
type ServiceRoute struct {
	PathPrefix  string // 路径前缀，如 "/admin", "/balance", "/game"
	ServiceName string // 对应的微服务名称
	Description string // 描述
}

// 总后台服务路由配置
var manageServiceRoutes = []ServiceRoute{
	{
		PathPrefix:  "/",
		ServiceName: "jh_manage_service",
		Description: "总后台相关接口",
	},
}

// 后台服务路由配置
var backendServiceRoutes = []ServiceRoute{
	{
		PathPrefix:  "/app",
		ServiceName: "jh_app_service",
		Description: "管理员相关接口",
	},
	{
		PathPrefix:  "/balance",
		ServiceName: "jh_balance_service",
		Description: "余额支付相关接口",
	},
	{
		PathPrefix:  "/game",
		ServiceName: "jh_game_service",
		Description: "游戏相关接口",
	},
}

// 前台服务路由配置（预留）
var frontendServiceRoutes = []ServiceRoute{
	{
		PathPrefix:  "/app",
		ServiceName: "jh_app_service", // 暂时复用app服务
		Description: "用户相关接口",
	},
	{
		PathPrefix:  "/game",
		ServiceName: "jh_game_service",
		Description: "游戏相关接口",
	},
	{
		PathPrefix:  "/balance",
		ServiceName: "jh_balance_service",
		Description: "支付相关接口",
	},
}

// ManageRouteDispatcher 总后台路由分发器
func ManageRouteDispatcher() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path
		method := r.Method

		util.LogWithTrace(ctx, "info", "=== ManageRouteDispatcher 开始处理请求: %s %s ===", method, path)

		// 创建HTTP请求的Jaeger span
		ctx, span := tracing.StartSpan(ctx, "manage.request", trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("route.type", "backend"),
		))
		defer span.End()

		util.LogWithTrace(ctx, "info", "Manage route dispatcher processing: %s %s", method, path)

		// 总后台全局报表直连处理（使用总后台token，不走后台token校验）
		if path == "/api/manage/operations/global-report" {
			HandleGlobalReport(ctx, r)
			return
		}

		// 总后台分析页补充接口：按站点透传到业务服务
		switch path {
		case "/api/manage/balance/recharge-payment":
			if err := injectManageSiteID(ctx, r); err != nil {
				util.WriteBadRequest(r, err.Error())
				return
			}
			if err := callServiceWithRouting(ctx, "jh_balance_service", r); err != nil {
				tracing.SetSpanError(span, err)
				util.WriteInternalError(r, err.Error())
				return
			}
			return
		case "/api/manage/balance/withdraws":
			if err := injectManageSiteID(ctx, r); err != nil {
				util.WriteBadRequest(r, err.Error())
				return
			}
			if err := callServiceWithRouting(ctx, "jh_balance_service", r); err != nil {
				tracing.SetSpanError(span, err)
				util.WriteInternalError(r, err.Error())
				return
			}
			return
		case "/api/manage/app/user-login-logs":
			if err := injectManageSiteID(ctx, r); err != nil {
				util.WriteBadRequest(r, err.Error())
				return
			}
			if err := callServiceWithRouting(ctx, "jh_app_service", r); err != nil {
				tracing.SetSpanError(span, err)
				util.WriteInternalError(r, err.Error())
				return
			}
			return
		}

		// 解析路径，确定目标服务
		serviceName, err := determineManageService(path)
		if err != nil {
			tracing.SetSpanError(span, err)
			util.WriteBadRequest(r, err.Error())
			return
		}

		util.LogWithTrace(ctx, "info", "=== 确定目标服务: %s ===", serviceName)
		tracing.SetSpanAttributes(span, attribute.String("target.service", serviceName))

		// 调用对应的服务处理器
		util.LogWithTrace(ctx, "info", "=== 准备调用服务处理器 ===")
		if err := callServiceWithRouting(ctx, serviceName, r); err != nil {
			tracing.SetSpanError(span, err)
			util.WriteInternalError(r, err.Error())
			return
		}

		util.LogWithTrace(ctx, "info", "=== 服务处理器调用完成 ===")
		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
		util.LogWithTrace(ctx, "info", "=== ManageRouteDispatcher 处理完成 ===")
	}
}

// injectManageSiteID 为总后台跨站点查询接口注入站点ID
func injectManageSiteID(ctx context.Context, r *ghttp.Request) error {
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		return fmt.Errorf("请求参数解析失败")
	}

	siteID := gconv.Int64(reqData["site_id"])
	if siteID == 0 {
		siteID = r.Get("site_id").Int64()
	}
	if siteID <= 0 {
		return fmt.Errorf("站点ID不能为空")
	}

	r.SetCtxVar("site_id", siteID)
	r.Header.Set("X-Site-Id", gconv.String(siteID))
	return nil
}

// BackendRouteDispatcher 后台路由分发器
func BackendRouteDispatcher() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path
		method := r.Method

		// 创建HTTP请求的Jaeger span
		ctx, span := tracing.StartSpan(ctx, "backend.request", trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("route.type", "backend"),
		))
		defer span.End()

		util.LogWithTrace(ctx, "info", "Backend route dispatcher processing: %s %s", method, path)

		// ClickHouse 全局报表直连处理
		if path == "/api/backend/balance/global-report" {
			HandleGlobalReport(ctx, r)
			return
		}

		// 解析路径，确定目标服务
		serviceName, err := determineBackendService(path)
		if err != nil {
			tracing.SetSpanError(span, err)
			util.WriteBadRequest(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.String("target.service", serviceName))

		// 调用对应的服务处理器
		if err := callServiceWithRouting(ctx, serviceName, r); err != nil {
			tracing.SetSpanError(span, err)
			util.WriteInternalError(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
	}
}

// FrontendRouteDispatcher 前台路由分发器
func FrontendRouteDispatcher() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path
		method := r.Method

		// 创建HTTP请求的Jaeger span
		ctx, span := tracing.StartSpan(ctx, "frontend.request", trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("route.type", "frontend"),
		))
		defer span.End()

		util.LogWithTrace(ctx, "info", "Frontend route dispatcher processing: %s %s", method, path)

		// 解析路径，确定目标服务
		serviceName, err := determineFrontendService(path)
		if err != nil {
			tracing.SetSpanError(span, err)
			util.WriteBadRequest(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.String("target.service", serviceName))

		// 调用对应的服务处理器
		if err := callServiceWithRouting(ctx, serviceName, r); err != nil {
			tracing.SetSpanError(span, err)
			util.WriteInternalError(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
	}
}

// BalanceRouteDispatcher 兼容路由分发器（/api/balance）
func BalanceRouteDispatcher() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path
		method := r.Method

		ctx, span := tracing.StartSpan(ctx, "balance.request", trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("route.type", "balance_legacy"),
		))
		defer span.End()

		util.LogWithTrace(ctx, "info", "Balance route dispatcher processing: %s %s", method, path)
		tracing.SetSpanAttributes(span, attribute.String("target.service", "jh_balance_service"))

		if err := callServiceWithRouting(ctx, "jh_balance_service", r); err != nil {
			tracing.SetSpanError(span, err)
			util.WriteInternalError(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
	}
}

// GameRouteDispatcher 兼容路由分发器（/api/game）
func GameRouteDispatcher() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path
		method := r.Method

		ctx, span := tracing.StartSpan(ctx, "game.request", trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("route.type", "game_legacy"),
		))
		defer span.End()

		util.LogWithTrace(ctx, "info", "Game route dispatcher processing: %s %s", method, path)
		tracing.SetSpanAttributes(span, attribute.String("target.service", "jh_game_service"))

		if err := callServiceWithRouting(ctx, "jh_game_service", r); err != nil {
			tracing.SetSpanError(span, err)
			util.WriteInternalError(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
	}
}

// determineManageService 根据路径确定总后台目标服务
func determineManageService(path string) (string, error) {
	// 移除 /api/manage 前缀
	path = strings.TrimPrefix(path, "/api/manage")

	// 查找匹配的服务路由
	for _, route := range manageServiceRoutes {
		if strings.HasPrefix(path, route.PathPrefix) {
			return route.ServiceName, nil
		}
	}

	return "", fmt.Errorf("no manage service found for path: %s", path)
}

// determineBackendService 根据路径确定后台目标服务
func determineBackendService(path string) (string, error) {
	// 移除 /api/backend 前缀
	path = strings.TrimPrefix(path, "/api/backend")

	// 查找匹配的服务路由
	for _, route := range backendServiceRoutes {
		if strings.HasPrefix(path, route.PathPrefix) {
			return route.ServiceName, nil
		}
	}

	return "", fmt.Errorf("no backend service found for path: %s", path)
}

// determineFrontendService 根据路径确定前台目标服务
func determineFrontendService(path string) (string, error) {
	// 移除 /api/frontend 前缀
	path = strings.TrimPrefix(path, "/api/frontend")

	// 查找匹配的服务路由
	for _, route := range frontendServiceRoutes {
		if strings.HasPrefix(path, route.PathPrefix) {
			return route.ServiceName, nil
		}
	}

	return "", fmt.Errorf("no frontend service found for path: %s", path)
}

// callServiceWithRouting 调用指定服务的路由处理
func callServiceWithRouting(ctx context.Context, serviceName string, r *ghttp.Request) error {
	// 从 Consul 获取服务地址
	ctx, discoverySpan := tracing.StartSpan(ctx, "service.discovery", trace.WithAttributes(
		attribute.String("service.name", serviceName),
	))
	addr, err := registry.GetServiceAddr(serviceName)
	discoverySpan.End()

	if err != nil {
		tracing.SetSpanError(discoverySpan, err)
		return fmt.Errorf("service [%s] not available: %v", serviceName, err)
	}

	tracing.SetSpanAttributes(discoverySpan, attribute.String("service.address", addr))
	util.LogWithTrace(ctx, "info", "Selected service instance: %s -> %s", serviceName, addr)

	// 连接到 gRPC 服务
	ctx, connSpan := tracing.StartSpan(ctx, "grpc.connect", trace.WithAttributes(
		attribute.String("grpc.target", addr),
	))
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	)
	connSpan.End()

	if err != nil {
		tracing.SetSpanError(connSpan, err)
		return fmt.Errorf("failed to connect to service [%s]: %v", serviceName, err)
	}
	defer conn.Close()

	// 根据服务名称调用对应的处理方法
	switch serviceName {
	case "jh_app_service":
		// 判断是前台还是后台请求
		if strings.HasPrefix(r.URL.Path, "/api/frontend") {
			return callFrontendGRPCMethod(ctx, conn, r)
		}
		return callBackendGRPCMethod(ctx, conn, r)
	case "jh_balance_service":
		return callBalanceGRPCMethod(ctx, conn, r)
	case "jh_game_service":
		return callGameGRPCMethod(ctx, conn, r)
	case "jh_manage_service":
		return callManageGRPCMethod(ctx, conn, r)
	default:
		return fmt.Errorf("unsupported service: %s", serviceName)
	}
}

// GetBackendServiceRoutes 获取后台服务路由配置（用于文档生成等）
func GetBackendServiceRoutes() []ServiceRoute {
	return backendServiceRoutes
}

// GetFrontendServiceRoutes 获取前台服务路由配置（用于文档生成等）
func GetFrontendServiceRoutes() []ServiceRoute {
	return frontendServiceRoutes
}

// injectUserInfoToMetadata 将HTTP请求上下文中的用户信息注入到gRPC metadata中
func injectUserInfoToMetadata(ctx context.Context, r *ghttp.Request) context.Context {
	md := metadata.New(map[string]string{})

	// 从HTTP Header中提取用户信息（由认证中间件设置）
	if userID := r.Header.Get("X-User-Id"); userID != "" {
		md.Set("user_id", userID)
	}

	if username := r.Header.Get("X-Username"); username != "" {
		md.Set("username", username)
	}

	if siteID := r.Header.Get("X-Site-Id"); siteID != "" {
		md.Set("site_id", siteID)
	}

	if adminID := r.Header.Get("X-Admin-Id"); adminID != "" {
		md.Set("admin_id", adminID)
	}

	// 从请求上下文中提取站点配置信息（由domain_resolver中间件设置）
	if siteCodeVar := r.GetCtxVar("site_code"); siteCodeVar != nil && siteCodeVar.String() != "" {
		md.Set("site_code", siteCodeVar.String())
	}

	if currencyVar := r.GetCtxVar("currency"); currencyVar != nil && currencyVar.String() != "" {
		md.Set("currency", currencyVar.String())
	}

	if domainVar := r.GetCtxVar("domain"); domainVar != nil && domainVar.String() != "" {
		md.Set("domain", domainVar.String())
	}

	// 传递客户端IP
	if clientIP := r.GetClientIp(); clientIP != "" {
		md.Set("client_ip", clientIP)
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
	if network := r.Header.Get("X-Network-Type"); network != "" {
		md.Set("x-network-type", network)
	}
	if screen := r.Header.Get("X-Screen-Resolution"); screen != "" {
		md.Set("x-screen-resolution", screen)
	}

	// 如果有现有的metadata，合并
	if existingMd, ok := metadata.FromOutgoingContext(ctx); ok {
		md = metadata.Join(existingMd, md)
	}

	return metadata.NewOutgoingContext(ctx, md)
}
