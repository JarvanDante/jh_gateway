package proxy

import (
	"jh_gateway/internal/registry"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func ToService(serviceName string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		r.SetCtxVar("serviceName", serviceName)

		addr, err := registry.GetServiceAddr(serviceName)
		if err != nil {
			util.WriteServiceUnavailable(r, "service not available")
			return
		}
		r.SetCtxVar("serviceAddr", addr)
		r.Middleware.Next()
	}
}

func Forward(r *ghttp.Request) {
	ctx := r.Context()
	addr := r.GetCtxVar("serviceAddr").String()

	targetURL := "http://" + addr + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	client := g.Client()
	// 把原请求的头部透传给下游（trace_id, user_id 等）
	for k, vals := range r.Header {
		if len(vals) > 0 {
			client.SetHeader(k, vals[0]) // 只取第一个值，一般够用了
		}
	}

	resp, err := client.DoRequest(ctx, r.Method, targetURL, r.Body)
	if err != nil {
		util.WriteError(r, 502, "upstream error: "+err.Error())
		return
	}
	defer resp.Close()

	// 状态码透传
	r.Response.WriteHeader(resp.StatusCode)

	// 响应头透传
	for k, vals := range resp.Header {
		for _, v := range vals {
			r.Response.Header().Add(k, v)
		}
	}

	// 响应 body 透传
	body := resp.ReadAll()
	r.Response.Write(body)
}
