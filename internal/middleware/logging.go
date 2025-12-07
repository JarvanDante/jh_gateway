package middleware

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func Logging(r *ghttp.Request) {
	start := time.Now()
	r.Middleware.Next()
	cost := time.Since(start)

	ctx := r.Context()
	traceID := r.GetCtxVar("traceId").String()
	g.Log().Info(ctx, "gateway_request",
		"path", r.URL.Path,
		"method", r.Method,
		"trace_id", traceID,
		"cost_ms", cost.Milliseconds(),
		"client_ip", r.GetClientIp(),
	)
}
