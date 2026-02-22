package middleware

import (
	"context"
	"fmt"
	"jh_gateway/internal/util"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

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
	path := r.URL.Path
	clientIP := r.GetClientIp()

	cfg := g.Cfg().MustGet(ctx, "rateLimit.requestsPerMinute").Int()
	if cfg <= 0 {
		r.Middleware.Next()
		return
	}

	bCfg := getBlockConfig(ctx)

	// 1) 先检查是否已被封禁
	blocked, err := isIPBlocked(ctx, clientIP, bCfg)
	if err != nil {
		g.Log().Warningf(ctx, "rateLimit block check failed: %v", err)
	} else if blocked {
		r.Response.Header().Set("X-RateLimit-Remaining", "0")
		r.Response.Header().Set("X-RateLimit-Blocked", "1")
		util.WriteTooManyRequests(r, "")
		return
	}

	// 2) 全局限流（不分 path）防整页F5狂刷
	globalCfg := g.Cfg().MustGet(ctx, "rateLimit.globalRequestsPerMinute").Int()
	if globalCfg <= 0 {
		globalCfg = cfg
	}
	globalKey := fmt.Sprintf("rate_limit:global:%s", getGlobalRateSubject(r))
	globalCount, globalReset, err := hitRedisCounter(ctx, globalKey, 60)
	if err != nil {
		g.Log().Warningf(ctx, "rateLimit global counter failed: %v", err)
	} else if globalCount > int64(globalCfg) {
		r.Response.Header().Set("X-RateLimit-Remaining", "0")
		r.Response.Header().Set("X-RateLimit-Reset", globalReset.Format(time.RFC3339))
		recordStrike(ctx, clientIP, bCfg)
		util.WriteTooManyRequests(r, "")
		return
	}

	// 3) 每接口限流（按 path 覆盖默认阈值）
	if routeCfg := getRouteLimit(path, ctx); routeCfg >= 0 {
		if routeCfg == 0 {
			r.Middleware.Next()
			return
		}
		cfg = routeCfg
	}

	pathKey := fmt.Sprintf("rate_limit:path:%s:%s", getGlobalRateSubject(r), path)
	count, reset, err := hitRedisCounter(ctx, pathKey, 60)
	if err != nil {
		g.Log().Warningf(ctx, "rateLimit path counter failed: %v", err)
		r.Middleware.Next()
		return
	}

	if count > int64(cfg) {
		r.Response.Header().Set("X-RateLimit-Remaining", "0")
		r.Response.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))
		recordStrike(ctx, clientIP, bCfg)
		util.WriteTooManyRequests(r, "")
		return
	}

	r.Response.Header().Set("X-RateLimit-Remaining", fmt.Sprint(cfg-int(count)))
	r.Response.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))
	r.Middleware.Next()
}

func getGlobalRateSubject(r *ghttp.Request) string {
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

func hitRedisCounter(ctx context.Context, key string, ttlSeconds int) (int64, time.Time, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}

	incrVar, err := g.Redis().Do(ctx, "INCR", key)
	if err != nil {
		return 0, time.Time{}, err
	}
	count := incrVar.Int64()

	if count == 1 {
		if _, err = g.Redis().Do(ctx, "EXPIRE", key, ttlSeconds); err != nil {
			return count, time.Now().Add(time.Duration(ttlSeconds) * time.Second), err
		}
	}

	ttlVar, err := g.Redis().Do(ctx, "TTL", key)
	if err != nil {
		return count, time.Now().Add(time.Duration(ttlSeconds) * time.Second), nil
	}
	ttl := ttlVar.Int()
	if ttl <= 0 {
		ttl = ttlSeconds
		_, _ = g.Redis().Do(ctx, "EXPIRE", key, ttlSeconds)
	}

	reset := time.Now().Add(time.Duration(ttl) * time.Second)
	return count, reset, nil
}

func getBlockConfig(ctx context.Context) blockConfig {
	cfg := blockConfig{
		Enabled:         true,
		Threshold:       5,
		WindowSeconds:   300,
		DurationSeconds: 1800,
	}
	_ = g.Cfg().MustGet(ctx, "rateLimit.block").Scan(&cfg)

	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 300
	}
	if cfg.DurationSeconds <= 0 {
		cfg.DurationSeconds = 1800
	}
	return cfg
}

func isIPBlocked(ctx context.Context, ip string, cfg blockConfig) (bool, error) {
	if !cfg.Enabled || ip == "" || isWhitelistedIP(ip, cfg.WhitelistIPs) {
		return false, nil
	}
	key := fmt.Sprintf("rate_limit:block:%s", ip)
	val, err := g.Redis().Get(ctx, key)
	if err != nil {
		return false, err
	}
	if val == nil || val.IsNil() {
		return false, nil
	}
	return strings.ToUpper(strings.TrimSpace(val.String())) == "BLOCKED", nil
}

func recordStrike(ctx context.Context, ip string, cfg blockConfig) {
	if !cfg.Enabled || ip == "" || isWhitelistedIP(ip, cfg.WhitelistIPs) {
		return
	}

	blockKey := fmt.Sprintf("rate_limit:block:%s", ip)

	// 单key方案：先用同一个 key 计数，达到阈值后再改写为 BLOCKED
	countVar, err := g.Redis().Do(ctx, "INCR", blockKey)
	if err != nil {
		g.Log().Warningf(ctx, "rateLimit strike incr failed: %v", err)
		return
	}
	count := countVar.Int()
	if count == 1 {
		_, _ = g.Redis().Do(ctx, "EXPIRE", blockKey, cfg.WindowSeconds)
	}

	if count >= cfg.Threshold {
		_, _ = g.Redis().Do(ctx, "SET", blockKey, "BLOCKED", "EX", cfg.DurationSeconds)
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

	for _, route := range routes {
		if route.ExactPath != "" && route.ExactPath == path {
			return route.RequestsPerMinute
		}
	}

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
