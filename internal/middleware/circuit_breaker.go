package middleware

import (
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

/*
 * 这是“土炮版熔断”，但已经能避免下游服务雪崩；以后可以升级成 Hystrix 模式或用专门库
 */

type breakerState struct {
	FailCount int
	OpenUntil time.Time
}

var (
	breakerMap   = make(map[string]*breakerState)
	breakerMutex sync.Mutex
)

// CircuitBreaker
// 这里简单示例：由下游服务通过 header 告诉网关本次是否失败，也可以根据 HTTP 状态码判断
func CircuitBreaker(r *ghttp.Request) {
	serviceName := r.GetCtxVar("serviceName").String()
	if serviceName == "" {
		r.Middleware.Next()
		return
	}

	now := time.Now()
	breakerMutex.Lock()
	state, ok := breakerMap[serviceName]
	if !ok {
		state = &breakerState{}
		breakerMap[serviceName] = state
	}
	if now.Before(state.OpenUntil) {
		breakerMutex.Unlock()
		r.Response.WriteJsonExit(g.Map{
			"code": 503,
			"msg":  "service temporarily unavailable",
		})
		return
	}
	breakerMutex.Unlock()

	// 放行
	r.Middleware.Next()

	// 简单示例：如果下游返回 5xx，则增加失败计数
	status := r.Response.Status
	if status >= 500 {
		breakerMutex.Lock()
		state.FailCount++
		if state.FailCount >= 5 {
			state.OpenUntil = time.Now().Add(10 * time.Second)
			state.FailCount = 0
			g.Log().Warning(r.Context(), "circuit breaker open for service", serviceName)
		}
		breakerMutex.Unlock()
	} else {
		// 成功则重置
		breakerMutex.Lock()
		state.FailCount = 0
		breakerMutex.Unlock()
	}
}
