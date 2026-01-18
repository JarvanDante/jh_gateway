package middleware

import (
	"fmt"
	"jh_gateway/internal/util"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// AuthWithSkip 创建一个可以跳过指定路径的认证中间件
func AuthWithSkip(skipPaths ...string) ghttp.HandlerFunc {
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
		claims, err := util.ParseToken(ctx, auth)
		if err != nil {
			util.LogWithTrace(ctx, "warn", "token验证失败: %s, error: %v", path, err)
			util.WriteUnauthorized(r, "token无效或已过期，请重新登录")
			return
		}

		// 判断是前台还是后台接口
		if strings.HasPrefix(path, "/api/frontend/") {
			// 前台接口：设置用户ID
			r.SetCtxVar("user_id", claims.UserID)
			r.Header.Set("X-User-Id", fmt.Sprint(claims.UserID))
			util.LogWithTrace(ctx, "info", "用户信息user_id放入上下文: %d", claims.UserID)
			util.LogWithTrace(ctx, "debug", "token验证成功: %s, userId: %d", path, claims.UserID)
		} else {
			// 后台接口：设置管理员ID
			r.SetCtxVar("admin_id", claims.AdminID)
			r.Header.Set("X-Admin-Id", fmt.Sprint(claims.AdminID))
			util.LogWithTrace(ctx, "info", "admin_service:管理员信息admin_id放入上下文: %d", claims.AdminID)
			util.LogWithTrace(ctx, "debug", "token验证成功: %s, adminId: %d", path, claims.AdminID)
		}

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
