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

	// 用户相关：转发到 user-service
	s.Group("/api/user", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			// middleware.JWTAuth,        // 暂时注释，用于测试
			middleware.CircuitBreaker, // 简单熔断
			proxy.ToService("user-service"),
		)
		group.ALL("/*any", proxy.Forward)
	})

	// 支付相关：转发到 payment-service
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

	// 游戏相关：转发到 game-service
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
