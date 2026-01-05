package router

import (
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/proxy"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
)

func Register(s *ghttp.Server) {
	// 健康检查
	s.BindHandler("/gateway/health", func(r *ghttp.Request) {
		util.WriteSuccess(r, nil)
	})

	// 管理员相关：通过 gRPC 调用 admin_service (包含管理员功能和站点设置)
	s.Group("/api/admin", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.RequestParser,          // 解析请求体数据
			middleware.AuthWithSkip("/login"), // 认证中间件（跳过 /login 接口）
			middleware.CircuitBreaker,
		)

		// 其他管理员接口：转发到 app-service (gRPC)
		group.ALL("/*any", proxy.AppGRPCToHTTP("jh_app_service"))
	})

	// 支付相关：转发到 balance-service (gRPC)
	s.Group("/api/balance", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.RequestParser,          // 解析请求体数据
			middleware.AuthWithSkip("/login"), // 认证中间件（跳过 /login 接口）
			middleware.CircuitBreaker,
		)
		group.ALL("/*any", proxy.BalanceGRPCToHTTP("jh_balance_service"))
	})

	// 支付相关：转发到 game-service (gRPC)
	s.Group("/api/game", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.RequestParser,          // 解析请求体数据
			middleware.AuthWithSkip("/login"), // 认证中间件（跳过 /login 接口）
			middleware.CircuitBreaker,
		)
		group.ALL("/*any", proxy.GameGRPCToHTTP("jh_game_service"))
	})
}
