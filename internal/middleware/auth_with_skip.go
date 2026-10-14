package middleware

import (
	"fmt"
	"jh_gateway/internal/util"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

func applyAuthClaims(r *ghttp.Request, claims *util.Claims) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/frontend/") {
		r.SetCtxVar("user_id", claims.UserID)
		r.SetCtxVar("username", claims.Username)
		r.SetCtxVar("site_id", claims.SiteID)
		r.Header.Set("X-User-Id", fmt.Sprint(claims.UserID))
		r.Header.Set("X-Username", claims.Username)
		r.Header.Set("X-Site-Id", fmt.Sprint(claims.SiteID))
		util.LogWithTrace(r.Context(), "info", "用户信息放入上下文: userId=%d, username=%s, siteId=%d", claims.UserID, claims.Username, claims.SiteID)
		return
	}

	r.SetCtxVar("admin_id", claims.AdminID)
	r.Header.Set("X-Admin-Id", fmt.Sprint(claims.AdminID))
	// 安全修复(A4)：把管理员 token 里的 site_id 作为「权威站点」向下游传递。
	r.SetCtxVar("admin_site_id", claims.SiteID)
	r.Header.Set("X-Admin-Site-Id", fmt.Sprint(claims.SiteID))
	util.LogWithTrace(r.Context(), "info", "admin_service:管理员信息放入上下文: admin_id=%d, site_id=%d", claims.AdminID, claims.SiteID)
}

func attachOptionalFrontendUser(r *ghttp.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return
	}
	claims, err := util.ParseToken(r.Context(), auth)
	if err != nil {
		return
	}
	applyAuthClaims(r, claims)
}

// AuthWithSkip 创建一个可以跳过指定路径的认证中间件
func AuthWithSkip(skipPaths ...string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path

		// 检查是否需要跳过认证（公开接口仍尽量解析 token，便于带上收藏等个性化字段）
		for _, skipPath := range skipPaths {
			if strings.HasSuffix(path, skipPath) {
				attachOptionalFrontendUser(r)
				util.LogWithTrace(ctx, "debug", "跳过强制认证: %s (匹配规则: %s)", path, skipPath)
				r.Middleware.Next()
				return
			}
		}

		// 检查 Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			util.LogWithTrace(ctx, "warn", "接口缺少token: %s", path)
			util.WriteUnauthorized(r, "缺少认证token，请先登录")
			return
		}

		// 解析和验证 token
		claims, err := util.ParseToken(ctx, auth)
		if err != nil {
			util.LogWithTrace(ctx, "warn", "token验证失败: %s, error: %v", path, err)
			util.WriteUnauthorized(r, "token无效或已过期，请重新登录")
			return
		}

		applyAuthClaims(r, claims)
		r.Middleware.Next()
	}
}

// AuthManageWithSkip 总后台-创建一个可以跳过指定路径的认证中间件
func AuthManageWithSkip(skipPaths ...string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path

		// 检查是否需要跳过认证
		for _, skipPath := range skipPaths {
			if strings.HasSuffix(path, skipPath) {
				util.LogWithTrace(ctx, "debug", "跳过认证检查: %s (匹配规则: %s)", path, skipPath)
				r.Middleware.Next()
				return
			}
		}

		// 检查 Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			util.LogWithTrace(ctx, "warn", "接口缺少token: %s", path)
			util.WriteUnauthorized(r, "缺少认证token，请先登录")
			return
		}

		// 解析和验证 token
		claims, err := util.ParseManageToken(ctx, auth)
		if err != nil {
			util.LogWithTrace(ctx, "warn", "token验证失败: %s, error: %v", path, err)
			util.WriteUnauthorized(r, "token无效或已过期，请重新登录")
			return
		}

		r.SetCtxVar("admin_id", claims.AdminID)
		r.Header.Set("X-Admin-Id", fmt.Sprint(claims.AdminID))
		util.LogWithTrace(ctx, "info", "总后台-admin_id放入上下文: %d", claims.AdminID)

		util.LogWithTrace(ctx, "debug", "token验证成功: %s, userId: %d", path, claims.UserID)

		r.Middleware.Next()
	}
}
