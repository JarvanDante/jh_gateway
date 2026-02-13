package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"jh_gateway/internal/util"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

var noncePattern = regexp.MustCompile(`^[A-Za-z0-9\-_:.]{8,128}$`)

type apiSignatureConfig struct {
	Enabled                bool
	Required               bool
	AllowedTimeSkewSeconds int
	NonceTTLSeconds        int
	DefaultAppID           string
	SkipPaths              []string
	AppSecrets             map[string]string
}

var errReplayNonce = errors.New("replay nonce")

// ApiSignature 请求签名校验中间件。
//
// Header 协议：
// - X-App-Id
// - X-Timestamp (秒或毫秒时间戳)
// - X-Nonce
// - X-Sign (hex 或 base64)
//
// 签名明文：
// METHOD + "\n" + PATH_WITH_QUERY + "\n" + TIMESTAMP + "\n" + NONCE + "\n" + SHA256_HEX(BODY)
// 使用 HMAC-SHA256(secret) 计算后与 X-Sign 比较。
func ApiSignature(r *ghttp.Request) {
	ctx := r.Context()

	cfg := loadAPISignatureConfig(ctx)
	if !cfg.Enabled || r.Method == "OPTIONS" {
		r.Middleware.Next()
		return
	}

	// 上传接口通常是 multipart，先跳过，避免影响现有上传链路。
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.Middleware.Next()
		return
	}

	path := r.URL.Path
	if shouldSkipSignaturePath(path, cfg.SkipPaths) {
		r.Middleware.Next()
		return
	}

	appID := strings.TrimSpace(r.Header.Get("X-App-Id"))
	timestamp := strings.TrimSpace(r.Header.Get("X-Timestamp"))
	nonce := strings.TrimSpace(r.Header.Get("X-Nonce"))
	sign := strings.TrimSpace(r.Header.Get("X-Sign"))

	// 灰度模式：required=false 时，客户端不带签名头则放行。
	if !cfg.Required && appID == "" && timestamp == "" && nonce == "" && sign == "" {
		r.Middleware.Next()
		return
	}

	if appID == "" {
		appID = cfg.DefaultAppID
	}

	if appID == "" || timestamp == "" || nonce == "" || sign == "" {
		util.WriteUnauthorized(r, "签名参数缺失")
		return
	}

	if !noncePattern.MatchString(nonce) {
		util.WriteUnauthorized(r, "nonce格式无效")
		return
	}

	parsedTimestamp, err := parseUnixTimestamp(timestamp)
	if err != nil {
		util.WriteUnauthorized(r, "时间戳格式无效")
		return
	}

	if !isTimestampInWindow(parsedTimestamp, cfg.AllowedTimeSkewSeconds) {
		util.WriteUnauthorized(r, "请求已过期")
		return
	}

	secret := cfg.AppSecrets[appID]
	if secret == "" {
		secret = cfg.AppSecrets[cfg.DefaultAppID]
	}
	if secret == "" {
		util.WriteUnauthorized(r, "签名应用不存在")
		return
	}

	requestTarget := path
	if rawQuery := r.URL.RawQuery; rawQuery != "" {
		requestTarget = requestTarget + "?" + rawQuery
	}

	bodyHash := sha256.Sum256(r.GetBody())
	payload := strings.Join([]string{
		strings.ToUpper(r.Method),
		requestTarget,
		timestamp,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")

	expected := signHMACSHA256Hex(payload, secret)
	if !isSignatureEqual(sign, expected) {
		util.WriteUnauthorized(r, "签名校验失败")
		return
	}

	if err = checkAndSetNonce(ctx, appID, nonce, cfg.NonceTTLSeconds); err != nil {
		if errors.Is(err, errReplayNonce) {
			util.WriteUnauthorized(r, "请求重复提交")
			return
		}
		util.WriteInternalError(r, "签名校验服务异常")
		return
	}

	r.SetCtxVar("app_id", appID)
	r.SetCtxVar("sign_verified", true)
	r.Middleware.Next()
}

func loadAPISignatureConfig(ctx context.Context) apiSignatureConfig {
	cfg := apiSignatureConfig{
		Enabled:                g.Cfg().MustGet(ctx, "apiSignature.enabled", false).Bool(),
		Required:               g.Cfg().MustGet(ctx, "apiSignature.required", false).Bool(),
		AllowedTimeSkewSeconds: g.Cfg().MustGet(ctx, "apiSignature.allowedTimeSkewSeconds", 300).Int(),
		NonceTTLSeconds:        g.Cfg().MustGet(ctx, "apiSignature.nonceTTLSeconds", 300).Int(),
		DefaultAppID:           g.Cfg().MustGet(ctx, "apiSignature.defaultAppId", "default").String(),
		SkipPaths:              nil,
		AppSecrets:             map[string]string{},
	}

	_ = g.Cfg().MustGet(ctx, "apiSignature.skipPaths").Scan(&cfg.SkipPaths)
	_ = g.Cfg().MustGet(ctx, "apiSignature.appSecrets").Scan(&cfg.AppSecrets)

	if cfg.AllowedTimeSkewSeconds <= 0 {
		cfg.AllowedTimeSkewSeconds = 300
	}
	if cfg.NonceTTLSeconds <= 0 {
		cfg.NonceTTLSeconds = 300
	}
	if cfg.DefaultAppID == "" {
		cfg.DefaultAppID = "default"
	}
	if cfg.AppSecrets == nil {
		cfg.AppSecrets = map[string]string{}
	}

	return cfg
}

func shouldSkipSignaturePath(path string, skipPaths []string) bool {
	for _, skip := range skipPaths {
		if skip == "" {
			continue
		}
		if strings.HasSuffix(path, skip) {
			return true
		}
	}
	return false
}

func parseUnixTimestamp(timestamp string) (time.Time, error) {
	tsInt, err := strconvParseInt64(timestamp)
	if err != nil {
		return time.Time{}, err
	}
	// 兼容毫秒时间戳
	if tsInt > 1_000_000_000_000 {
		tsInt = tsInt / 1000
	}
	return time.Unix(tsInt, 0), nil
}

func isTimestampInWindow(requestTime time.Time, skewSeconds int) bool {
	now := time.Now()
	diff := now.Sub(requestTime)
	if diff < 0 {
		diff = -diff
	}
	return diff <= time.Duration(skewSeconds)*time.Second
}

func checkAndSetNonce(ctx context.Context, appID, nonce string, ttlSeconds int) error {
	key := fmt.Sprintf("api_sign_nonce:%s:%s", appID, nonce)
	resp, err := g.Redis().Do(ctx, "SET", key, "1", "NX", "EX", ttlSeconds)
	if err != nil {
		return err
	}
	if resp == nil {
		return errReplayNonce
	}
	if strings.ToUpper(strings.TrimSpace(resp.String())) != "OK" {
		return errReplayNonce
	}
	return nil
}

func signHMACSHA256Hex(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func isSignatureEqual(got, expectedHex string) bool {
	gotBytes, ok := decodeSignature(got)
	if !ok {
		return false
	}
	expectedBytes, err := hex.DecodeString(expectedHex)
	if err != nil {
		return false
	}
	return hmac.Equal(gotBytes, expectedBytes)
}

func decodeSignature(sign string) ([]byte, bool) {
	trimmed := strings.TrimSpace(sign)
	if trimmed == "" {
		return nil, false
	}

	// 1) hex
	if b, err := hex.DecodeString(trimmed); err == nil {
		return b, true
	}

	// 2) standard base64
	if b, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return b, true
	}

	// 3) raw base64
	if b, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil {
		return b, true
	}

	return nil, false
}

func strconvParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
