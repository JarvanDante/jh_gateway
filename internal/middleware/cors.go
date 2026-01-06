package middleware

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// CORS 跨域中间件 - 允许所有域名
func CORS(r *ghttp.Request) {
	// 设置 CORS 响应头 - 允许所有域名
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	r.Response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	r.Response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
	r.Response.Header().Set("Access-Control-Max-Age", "86400") // 24小时

	// 处理 OPTIONS 预检请求
	if r.Method == "OPTIONS" {
		r.Response.WriteHeader(200)
		return
	}

	// 继续处理请求
	r.Middleware.Next()
}

// CORSWithCredentials 允许所有域名并支持凭证的 CORS 中间件
func CORSWithCredentials(r *ghttp.Request) {
	// 获取请求的 Origin
	origin := r.Header.Get("Origin")

	// 如果有 Origin，则设置为该 Origin（支持凭证时不能使用 *）
	if origin != "" {
		r.Response.Header().Set("Access-Control-Allow-Origin", origin)
		r.Response.Header().Set("Access-Control-Allow-Credentials", "true")
	} else {
		r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	}

	r.Response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
	r.Response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
	r.Response.Header().Set("Access-Control-Max-Age", "86400")

	// 处理 OPTIONS 预检请求
	if r.Method == "OPTIONS" {
		r.Response.WriteHeader(200)
		return
	}

	r.Middleware.Next()
}
