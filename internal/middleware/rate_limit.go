package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

var (
	rateMap   = make(map[string]*rateCounter)
	rateMutex sync.Mutex
)

type rateCounter struct {
	Count int
	Reset time.Time
}

func RateLimit(r *ghttp.Request) {
	ctx := r.Context()
	cfg := g.Cfg().MustGet(ctx, "rateLimit.requestsPerMinute").Int()
	if cfg <= 0 {
		r.Middleware.Next()
		return
	}

	ip := r.GetClientIp()
	now := time.Now()

	rateMutex.Lock()
	rc, ok := rateMap[ip]
	if !ok || now.After(rc.Reset) {
		rc = &rateCounter{Count: 0, Reset: now.Add(time.Minute)}
		rateMap[ip] = rc
	}
	rc.Count++
	count := rc.Count
	reset := rc.Reset
	rateMutex.Unlock()

	if count > cfg {
		r.Response.WriteStatus(429)
		r.Response.WriteJsonExit(g.Map{
			"code": 429,
			"msg":  "too many requests",
		})
		return
	}

	// 返回剩余次数和重置时间（可选）
	r.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprint(cfg-count))
	r.Response.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))

	r.Middleware.Next()
}
