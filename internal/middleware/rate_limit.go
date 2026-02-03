package middleware

import (
	"context"
	"fmt"
	"jh_gateway/internal/util"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

var (
	rateMap   = make(map[string]*rateCounter)
	rateMutex sync.RWMutex
	cleanOnce sync.Once
)

type rateCounter struct {
	mu       sync.Mutex
	count    int
	reset    time.Time
	lastSeen time.Time
}

type routeLimit struct {
	ExactPath         string `json:"exactPath" yaml:"exactPath"`
	PathPrefix        string `json:"pathPrefix" yaml:"pathPrefix"`
	RequestsPerMinute int    `json:"requestsPerMinute" yaml:"requestsPerMinute"`
}

func RateLimit(r *ghttp.Request) {
	ctx := r.Context()
	cfg := g.Cfg().MustGet(ctx, "rateLimit.requestsPerMinute").Int()
	if cfg <= 0 {
		r.Middleware.Next()
		return
	}

	// 按接口配置覆盖默认阈值
	if routeCfg := getRouteLimit(r.URL.Path, ctx); routeCfg >= 0 {
		if routeCfg == 0 {
			// 对该接口禁用限流
			r.Middleware.Next()
			return
		}
		cfg = routeCfg
	}

	// 启动过期清理协程（只启动一次）
	cleanOnce.Do(func() { go cleanupRateMap() })

	key := getRateKey(r)
	now := time.Now()

	// 先读锁查找，降低锁竞争
	rateMutex.RLock()
	rc, ok := rateMap[key]
	rateMutex.RUnlock()

	if !ok {
		// 双重检查，避免并发创建重复
		rateMutex.Lock()
		rc, ok = rateMap[key]
		if !ok {
			rc = &rateCounter{count: 0, reset: now.Add(time.Minute), lastSeen: now}
			rateMap[key] = rc
		}
		rateMutex.Unlock()
	}

	rc.mu.Lock()
	if now.After(rc.reset) {
		rc.count = 0
		rc.reset = now.Add(time.Minute)
	}
	rc.count++
	rc.lastSeen = now
	count := rc.count
	reset := rc.reset
	rc.mu.Unlock()

	if count > cfg {
		r.Response.Header().Set("X-RateLimit-Remaining", "0")
		r.Response.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))
		util.WriteTooManyRequests(r, "")
		return
	}

	r.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprint(cfg-count))
	r.Response.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))

	r.Middleware.Next()
}

// getRateKey 优先按用户维度限流，未登录则回退到IP
func getRateKey(r *ghttp.Request) string {
	if userIdVar := r.GetCtxVar("user_id"); userIdVar != nil {
		if userID := userIdVar.Int64(); userID > 0 {
			return fmt.Sprintf("user:%d", userID)
		}
	}
	if adminIdVar := r.GetCtxVar("admin_id"); adminIdVar != nil {
		if adminID := adminIdVar.Int64(); adminID > 0 {
			return fmt.Sprintf("admin:%d", adminID)
		}
	}
	return fmt.Sprintf("ip:%s", r.GetClientIp())
}

// getRouteLimit 返回匹配到的接口限流值（精确匹配优先，其次最长前缀匹配）
// 返回值: >0 使用该值; 0 表示该接口禁用限流; -1 表示无匹配使用默认值
func getRouteLimit(path string, ctx context.Context) int {
	var routes []routeLimit
	if err := g.Cfg().MustGet(ctx, "rateLimit.routes").Scan(&routes); err != nil {
		return -1
	}
	if len(routes) == 0 {
		return -1
	}

	// 先精确匹配
	for _, route := range routes {
		if route.ExactPath != "" && route.ExactPath == path {
			return route.RequestsPerMinute
		}
	}

	// 再最长前缀匹配
	bestLen := -1
	bestLimit := -1
	for _, route := range routes {
		if route.PathPrefix == "" {
			continue
		}
		if strings.HasPrefix(path, route.PathPrefix) {
			if len(route.PathPrefix) > bestLen {
				bestLen = len(route.PathPrefix)
				bestLimit = route.RequestsPerMinute
			}
		}
	}

	if bestLen == -1 {
		return -1
	}
	return bestLimit
}

// cleanupRateMap 定期清理过期IP计数，避免内存增长
func cleanupRateMap() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		rateMutex.Lock()
		for ip, rc := range rateMap {
			rc.mu.Lock()
			expired := now.After(rc.reset.Add(5 * time.Minute))
			rc.mu.Unlock()
			if expired {
				delete(rateMap, ip)
			}
		}
		rateMutex.Unlock()
	}
}
