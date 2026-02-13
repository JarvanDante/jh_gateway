package proxy

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"jh_gateway/internal/middleware"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

type signSessionPayload struct {
	AppID     string `json:"app_id"`
	Secret    string `json:"secret"`
	TokenHash string `json:"token_hash,omitempty"`
	UserID    int    `json:"user_id,omitempty"`
	SiteID    int    `json:"site_id,omitempty"`
}

// FrontendSignSessionHandler 生成前台短期签名会话密钥
func FrontendSignSessionHandler() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()

		reqData, _ := middleware.GetRequestDataWithFallback(ctx, r)
		appID := strings.TrimSpace(fmt.Sprint(reqData["app_id"]))
		if appID == "" {
			appID = strings.TrimSpace(r.Header.Get("X-App-Id"))
		}
		if appID == "" {
			appID = strings.TrimSpace(g.Cfg().MustGet(ctx, "apiSignature.defaultAppId", "default").String())
		}
		if appID == "" {
			appID = "default"
		}

		ttlSeconds := g.Cfg().MustGet(ctx, "apiSignature.frontendSessionTTLSeconds", 600).Int()
		if ttlSeconds < 60 {
			ttlSeconds = 600
		}

		keyID, err := randomHex(16)
		if err != nil {
			util.WriteInternalError(r, "生成签名会话失败")
			return
		}
		signKey, err := randomHex(32)
		if err != nil {
			util.WriteInternalError(r, "生成签名会话失败")
			return
		}

		payload := signSessionPayload{
			AppID:  appID,
			Secret: signKey,
		}

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader != "" {
			claims, parseErr := util.ParseToken(ctx, authHeader)
			if parseErr != nil {
				// token 失效时回退为匿名签名会话，避免公共页面请求被 401 卡死
				g.Log().Warningf(ctx, "sign-session token invalid, fallback to anonymous session: %v", parseErr)
			} else {
				token, ok := extractBearerToken(authHeader)
				if !ok {
					g.Log().Warning(ctx, "sign-session authorization header invalid format, fallback to anonymous session")
				} else {
					payload.TokenHash = sha256Hex(token)
					payload.UserID = claims.UserID
					payload.SiteID = claims.SiteID
				}
			}
		}

		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			util.WriteInternalError(r, "生成签名会话失败")
			return
		}

		cacheKey := fmt.Sprintf("api_sign_session:%s", keyID)
		if _, err = g.Redis().Do(ctx, "SET", cacheKey, string(payloadJSON), "EX", ttlSeconds); err != nil {
			util.WriteInternalError(r, "签名会话存储失败")
			return
		}

		util.WriteSuccess(r, map[string]interface{}{
			"key_id":     keyID,
			"sign_key":   signKey,
			"app_id":     appID,
			"expires_in": ttlSeconds,
		})
	}
}

func randomHex(byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", fmt.Errorf("invalid byte length")
	}
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func extractBearerToken(authHeader string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(authHeader), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func sha256Hex(text string) string {
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}
