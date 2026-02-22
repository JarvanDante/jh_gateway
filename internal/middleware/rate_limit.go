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

	blockMap   = make(map[string]time.Time)
	blockMutex sync.RWMutex
	strikeMap  = make(map[string]*strikeCounter)
)

type rateCounter struct {
	mu       sync.Mutex
	count    int
	reset    time.Time
	lastSeen time.Time
}

type strikeCounter struct {
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

type blockConfig struct {
	Enabled         bool     `json:"enabled" yaml:"enabled"`
	Threshold       int      `json:"threshold" yaml:"threshold"`
	WindowSeconds   int      `json:"windowSeconds" yaml:"windowSeconds"`
	DurationSeconds int      `json:"durationSeconds" yaml:"durationSeconds"`
	WhitelistIPs    []string `json:"whitelistIPs" yaml:"whitelistIPs"`
}

func RateLimit(r *ghttp.Request) {
	ctx := r.Context()
	cfg := g.Cfg().MustGet(ctx, "rateLimit.requestsPerMinute").Int()
	if cfg <= 0 {
		r.Middleware.Next()
		return
	}

	path := r.URL.Path
	clientIP := r.GetClientIp()
	bCfg := getBlockConfig(ctx)

	// IP 黑名单拦截（优先）
	if shouldBlock(clientIP, bCfg) {
		r.Response.Header().Set("X-RateLimit-Remaining", "0")
		r.Response.Header().Set("X-RateLimit-Blocked", "1")
		util.WriteTooManyRequests(r, "")
		return
	}

	// 启动过期清理协程（只启动一次）
	cleanOnce.Do(func() { go cleanupRateMap() })
	now := time.Now()

	// 全局限流（按用户/IP，不分接口）——用于拦截整页F5狂刷
	globalCfg := g.Cfg().MustGet(ctx, "rateLimit.globalRequestsPerMinute").Int()
	if globalCfg <= 0 {
		globalCfg = cfg
	}
	globalKey := getGlobalRateKey(r)
	globalCount, globalReset := hitRateCounter(globalKey, now)
	if globalCount > globalCfg {
		r.Response.Header().Set("X-RateLimit-Remaining", "0")
		r.Response.Header().Set("X-RateLimit-Reset", globalReset.Format(time.RFC3339))
		recordStrike(clientIP, bCfg)
		util.WriteTooManyRequests(r, "")
		return
	}

	// 按接口配置覆盖默认阈值
	if routeCfg := getRouteLimit(path, ctx); routeCfg >= 0 {
		if routeCfg == 0 {
			// 对该接口禁用限流
			r.Middleware.Next()
			return
		}
		cfg = routeCfg
	}

	// 每接口限流（按用户/IP + path）
	key := getRateKey(r, path)
	count, reset := hitRateCounter(key, now)
	if count > cfg {
		r.Response.Header().Set("X-RateLimit-Remaining", "0")
		r.Response.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))
		recordStrike(clientIP, bCfg)
		util.WriteTooManyRequests(r, "")
		return
	}

	// 返回每接口剩余配额
	r.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprint(cfg-count))
	r.Response.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))

	r.Middleware.Next()
}

// getRateKey 优先按用户维度限流，未登录则回退到IP；并追加 path 做“每接口独立限流”
func getRateKey(r *ghttp.Request, path string) string {
	if userIdVar := r.GetCtxVar("user_id"); userIdVar != nil {
		if userID := userIdVar.Int64(); userID > 0 {
			return fmt.Sprintf("user:%d:path:%s", userID, path)
		}
	}
	if adminIdVar := r.GetCtxVar("admin_id"); adminIdVar != nil {
		if adminID := adminIdVar.Int64(); adminID > 0 {
			return fmt.Sprintf("admin:%d:path:%s", adminID, path)
		}
	}
	return fmt.Sprintf("ip:%s:path:%s", r.GetClientIp(), path)
}

// getGlobalRateKey 全局限流key（不区分path）
func getGlobalRateKey(r *ghttp.Request) string {
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

// hitRateCounter 统一处理分钟窗口计数
func hitRateCounter(key string, now time.Time) (count int, reset time.Time) {
	rateMutex.RLock()
	rc, ok := rateMap[key]
	rateMutex.RUnlock()

	if !ok {
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
	count = rc.count
	reset = rc.reset
	rc.mu.Unlock()
	return
}

func getBlockConfig(ctx context.Context) blockConfig {
	cfg := blockConfig{
		Enabled:         true,
		Threshold:       8,
		WindowSeconds:   300,
		DurationSeconds: 1800,
	}
	_ = g.Cfg().MustGet(ctx, "rateLimit.block").Scan(&cfg)

	if cfg.Threshold <= 0 {
		cfg.Threshold = 8
	}
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 300
	}
	if cfg.DurationSeconds <= 0 {
		cfg.DurationSeconds = 1800
	}
	return cfg
}

func shouldBlock(ip string, cfg blockConfig) bool {
	if !cfg.Enabled || ip == "" {
		return false
	}
	if isWhitelistedIP(ip, cfg.WhitelistIPs) {
		return false
	}

	now := time.Now()
	blockMutex.RLock()
	until, ok := blockMap[ip]
	blockMutex.RUnlock()
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}

	// 过期自动清理
	blockMutex.Lock()
	delete(blockMap, ip)
	blockMutex.Unlock()
	return false
}

func recordStrike(ip string, cfg blockConfig) {
	if !cfg.Enabled || ip == "" {
		return
	}
	if isWhitelistedIP(ip, cfg.WhitelistIPs) {
		return
	}

	now := time.Now()
	window := time.Duration(cfg.WindowSeconds) * time.Second

	blockMutex.RLock()
	sc, ok := strikeMap[ip]
	blockMutex.RUnlock()
	if !ok {
		blockMutex.Lock()
		sc, ok = strikeMap[ip]
		if !ok {
			sc = &strikeCounter{count: 0, reset: now.Add(window), lastSeen: now}
			strikeMap[ip] = sc
		}
		blockMutex.Unlock()
	}

	sc.mu.Lock()
	if now.After(sc.reset) {
		sc.count = 0
		sc.reset = now.Add(window)
	}
	sc.count++
	sc.lastSeen = now
	count := sc.count
	sc.mu.Unlock()

	if count >= cfg.Threshold {
		blockUntil := now.Add(time.Duration(cfg.DurationSeconds) * time.Second)
		blockMutex.Lock()
		blockMap[ip] = blockUntil
		delete(strikeMap, ip)
		blockMutex.Unlock()
	}
}

func isWhitelistedIP(ip string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return false
	}
	for _, w := range whitelist {
		if strings.TrimSpace(w) == ip {
			return true
		}
	}
	return false
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

// cleanupRateMap 定期清理过期计数与封禁状态，避免内存增长
func cleanupRateMap() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		rateMutex.Lock()
		for key, rc := range rateMap {
			rc.mu.Lock()
			expired := now.After(rc.reset.Add(5 * time.Minute))
			rc.mu.Unlock()
			if expired {
				delete(rateMap, key)
			}
		}
		rateMutex.Unlock()

		blockMutex.Lock()
		for ip, until := range blockMap {
			if now.After(until.Add(5 * time.Minute)) {
				delete(blockMap, ip)
			}
		}
		for ip, sc := range strikeMap {
			sc.mu.Lock()
			expired := now.After(sc.reset.Add(5 * time.Minute))
			sc.mu.Unlock()
			if expired {
				delete(strikeMap, ip)
			}
		}
		blockMutex.Unlock()
	}
}
