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
		group.ALL("/*any", proxy.AdminGRPCToHTTP("admin_service"))
	})

	// 支付相关：转发到 payment-service (HTTP)
	s.Group("/api/pay", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.JWTAuth,
			middleware.CircuitBreaker,
			proxy.ToService("payment-service"),
		)
		group.ALL("/*any", proxy.Forward)
	})

	// 游戏相关：转发到 game-service (HTTP)
	s.Group("/api/game", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.JWTAuth,
			middleware.CircuitBreaker,
			proxy.ToService("game-service"),
		)
		group.ALL("/*any", proxy.Forward)
	})

}
