package router

import (
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/proxy"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
)

func Register(s *ghttp.Server) {
	// 健康检查
	s.BindHandler("/gateway/health", func(r *ghttp.Request) {
		util.WriteSuccess(r, nil)
	})

	// 域名映射调试接口
	s.BindHandler("/gateway/domain-mappings", func(r *ghttp.Request) {
		mappings := registry.GetAllDomainMappings()
		stats := registry.GetDomainStats()

		util.WriteSuccess(r, map[string]interface{}{
			"mappings": mappings,
			"stats":    stats,
		})
	})

	// 总后台管理接口统一入口
	s.Group("/api/manage", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.CORS, // CORS 跨域中间件
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.RequestParser, // 解析请求体数据
			middleware.AuthManageWithSkip("/login", "/logout"), // 认证中间件（跳过 /login 接口）
			middleware.CircuitBreaker,
		)

		// 后台接口路由分发
		group.ALL("/*any", proxy.ManageRouteDispatcher())
	})

	// 后台管理接口统一入口
	s.Group("/api/backend", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.CORS,           // CORS 跨域中间件
			middleware.DomainResolver, // 域名解析中间件
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.RequestParser,                     // 解析请求体数据
			middleware.AuthWithSkip("/login", "/logout"), // 认证中间件（跳过 /login 接口）
			middleware.CircuitBreaker,
		)

		// 后台接口路由分发
		group.ALL("/*any", proxy.BackendRouteDispatcher())
	})

	// 前台用户接口（预留）
	s.Group("/api/frontend", func(group *ghttp.RouterGroup) {
		group.Middleware(
			middleware.CORS,           // CORS 跨域中间件
			middleware.DomainResolver, // 域名解析中间件
			middleware.Logging,
			middleware.Trace,
			middleware.RateLimit,
			middleware.RequestParser, // 解析请求体数据
			middleware.AuthWithSkip("/game-categories", "/notice-list", "/ad-list", "/quickRegister", "/register", "/captcha", "/refresh-token", "/login", "/logout"), // 认证中间件（跳过 /login 接口）
			middleware.CircuitBreaker,
		)

		// 前台接口路由分发（暂时返回未实现）
		group.ALL("/*any", proxy.FrontendRouteDispatcher())
	})

	// 兼容性路由（可选，用于渐进式迁移）
	registerLegacyRoutes(s)
}

// registerLegacyRoutes 注册兼容性路由，便于渐进式迁移
func registerLegacyRoutes(s *ghttp.Server) {
	// 如果需要保持向后兼容，可以保留原有路由并重定向到新路由
	// 这里暂时注释掉，如果需要可以启用

	/*
		// 管理员相关：重定向到后台统一入口
		s.Group("/api/backend/app", func(group *ghttp.RouterGroup) {
			group.Middleware(
				middleware.Logging,
				middleware.Trace,
				middleware.RateLimit,
				middleware.RequestParser,
				middleware.AuthWithSkip("/login"),
				middleware.CircuitBreaker,
			)
			group.ALL("/*any", proxy.AppGRPCToHTTP("jh_app_service"))
		})

		// 支付相关：重定向到后台统一入口
		s.Group("/api/backend/balance", func(group *ghttp.RouterGroup) {
			group.Middleware(
				middleware.Logging,
				middleware.Trace,
				middleware.RateLimit,
				middleware.RequestParser,
				middleware.AuthWithSkip("/login"),
				middleware.CircuitBreaker,
			)
			group.ALL("/*any", proxy.BalanceGRPCToHTTP("jh_balance_service"))
		})

		// 游戏相关：重定向到后台统一入口
		s.Group("/api/backend/game", func(group *ghttp.RouterGroup) {
			group.Middleware(
				middleware.Logging,
				middleware.Trace,
				middleware.RateLimit,
				middleware.RequestParser,
				middleware.AuthWithSkip("/login"),
				middleware.CircuitBreaker,
			)
			group.ALL("/*any", proxy.GameGRPCToHTTP("jh_game_service"))
		})
	*/
}
