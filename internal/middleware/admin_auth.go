package middleware

import (
	"fmt"
	"jh_gateway/internal/util"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// AdminAuth 管理员认证中间件
// 除了 /api/admin/login 接口外，其他接口都需要 token 验证
func AdminAuth(r *ghttp.Request) {
	ctx := r.Context()
	path := r.URL.Path

	// 跳过登录接口
	if strings.HasSuffix(path, "/login") {
		util.LogWithTrace(ctx, "debug", "跳过登录接口的token验证: %s", path)
		r.Middleware.Next()
		return
	}

	// 检查 Authorization header
	auth := r.Header.Get("Authorization")
	if auth == "" {
		util.LogWithTrace(ctx, "warn", "管理员接口缺少token: %s", path)
		util.WriteUnauthorized(r, "缺少认证token，请先登录")
		return
	}

	// 解析和验证 token
	claims, err := util.ParseToken(ctx, auth)
	if err != nil {
		util.LogWithTrace(ctx, "warn", "管理员token验证失败: %s, error: %v", path, err)
		util.WriteUnauthorized(r, "token无效或已过期，请重新登录")
		return
	}

	// 验证成功，将用户信息放入上下文和请求头
	r.SetCtxVar("adminId", claims.UserID)
	r.Header.Set("X-Admin-Id", fmt.Sprint(claims.UserID))

	util.LogWithTrace(ctx, "debug", "管理员token验证成功: %s, adminId: %d", path, claims.UserID)

	r.Middleware.Next()
}
