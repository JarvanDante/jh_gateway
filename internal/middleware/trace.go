package middleware

import (
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
)

func Trace(r *ghttp.Request) {
	traceID := r.Header.Get("X-Trace-Id")
	if traceID == "" {
		traceID = util.NewTraceID()
	}
	r.Header.Set("X-Trace-Id", traceID)
	r.SetCtxVar("traceId", traceID)
	r.Middleware.Next()
}
