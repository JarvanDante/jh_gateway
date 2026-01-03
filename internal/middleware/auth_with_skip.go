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

		// 如果是管理员相关接口，也设置管理员ID
		if strings.HasPrefix(path, "/api/admin") {
			r.SetCtxVar("admin_id", claims.AdminID)
			r.Header.Set("X-Admin-Id", fmt.Sprint(claims.AdminID))
			util.LogWithTrace(ctx, "info", "admin_service:管理员信息admin_id放入上下文: %d", claims.AdminID)
		} else if strings.HasPrefix(path, "/api/balance") {
			r.SetCtxVar("admin_id", claims.AdminID)
			r.Header.Set("X-Admin-Id", fmt.Sprint(claims.AdminID))
			util.LogWithTrace(ctx, "info", "balance_service:管理员信息admin_id放入上下文: %d", claims.AdminID)
		} else {
			// 验证成功，将用户信息放入上下文和请求头
			r.SetCtxVar("user_id", claims.UserID)
			r.Header.Set("X-User-Id", fmt.Sprint(claims.UserID))
			util.LogWithTrace(ctx, "info", "用户信息user_id放入上下文: %d", claims.UserID)
		}

		util.LogWithTrace(ctx, "debug", "token验证成功: %s, userId: %d", path, claims.UserID)

		r.Middleware.Next()
	}
}
