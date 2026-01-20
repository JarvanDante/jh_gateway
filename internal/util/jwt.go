package util

import (
	"context"
	"fmt"
	"github.com/gogf/gf/v2/frame/g"
	jwt "github.com/golang-jwt/jwt/v5" // ⭐ 必须有这一行
	"strings"
)

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	SiteID   int    `json:"site_id"`
	AdminID  int    `json:"admin_id"`
	jwt.RegisteredClaims
}

func ParseToken(ctx context.Context, authHeader string) (*Claims, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, fmt.Errorf("invalid Authorization header format")
	}

	tokenStr := parts[1]
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()

	// 先用MapClaims解析，以便获取jti
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// 检查token是否在黑名单中（使用jti）
	if mapClaims, ok := token.Claims.(jwt.MapClaims); ok {
		if jti, exists := mapClaims["jti"]; exists {
			jtiStr, _ := jti.(string)
			if jtiStr != "" {
				blacklistKey := fmt.Sprintf("token_blacklist:%s", jtiStr)
				isBlacklisted, err := g.Redis().Exists(ctx, blacklistKey)
				if err == nil && isBlacklisted > 0 {
					return nil, fmt.Errorf("token已失效，请重新登录或刷新token")
				}
			}
		}

		// 从MapClaims提取Claims结构体需要的字段
		claims := &Claims{}
		if userID, ok := mapClaims["user_id"].(float64); ok {
			claims.UserID = int(userID)
		}
		if username, ok := mapClaims["username"].(string); ok {
			claims.Username = username
		}
		if siteID, ok := mapClaims["site_id"].(float64); ok {
			claims.SiteID = int(siteID)
		}
		if adminID, ok := mapClaims["admin_id"].(float64); ok {
			claims.AdminID = int(adminID)
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

func ParseManageToken(ctx context.Context, authHeader string) (*Claims, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, fmt.Errorf("invalid Authorization header format")
	}

	tokenStr := parts[1]
	secret := g.Cfg().MustGet(ctx, "jwt.manageSecret").String()

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

//func GenerateToken(userID int, secret string) (string, error) {
//	claims := Claims{
//		UserID: userID,
//		RegisteredClaims: jwt.RegisteredClaims{
//			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
//			IssuedAt:  jwt.NewNumericDate(time.Now()),
//		},
//	}
//
//	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
//	return token.SignedString([]byte(secret))
//}
