package util

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	jwt "github.com/golang-jwt/jwt/v5" // ⭐ 必须有这一行
)

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

func ParseToken(ctx context.Context, authHeader string) (*Claims, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, fmt.Errorf("invalid Authorization header format")
	}

	tokenStr := parts[1]
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()

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

func GenerateToken(userID int, secret string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
