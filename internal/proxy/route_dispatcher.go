package proxy

import (
	"context"
	"fmt"
	"strings"

	"jh_gateway/internal/registry"
	"jh_gateway/internal/tracing"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ServiceRoute 服务路由配置
type ServiceRoute struct {
	PathPrefix  string // 路径前缀，如 "/admin", "/balance", "/game"
	ServiceName string // 对应的微服务名称
	Description string // 描述
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

		// 暂时返回未实现，后续可以实现具体的前台逻辑
		util.WriteSuccess(r, map[string]interface{}{
			"message": "Frontend API not implemented yet",
			"path":    path,
			"method":  method,
			"service": serviceName,
			"status":  "coming_soon",
		})

		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
	}
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
		return callAppGRPCMethod(ctx, conn, r)
	case "jh_balance_service":
		return callBalanceGRPCMethod(ctx, conn, r)
	case "jh_game_service":
		return callGameGRPCMethod(ctx, conn, r)
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
