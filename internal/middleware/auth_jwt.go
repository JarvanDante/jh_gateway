package middleware

import (
	"fmt"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func JWTAuth(r *ghttp.Request) {
	ctx := r.Context()
	auth := r.Header.Get("Authorization")
	if auth == "" {
		r.Response.WriteJsonExit(g.Map{"code": 401, "msg": "missing token"})
		return
	}

	claims, err := util.ParseToken(ctx, auth)
	if err != nil {
		r.Response.WriteJsonExit(g.Map{"code": 401, "msg": "invalid token"})
		return
	}

	// 放入上下文，后端微服务也可以从 header 中拿
	r.SetCtxVar("userId", claims.UserID)
	r.Header.Set("X-User-Id", fmt.Sprint(claims.UserID))

	r.Middleware.Next()
}
