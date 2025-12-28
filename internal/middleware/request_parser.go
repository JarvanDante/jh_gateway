package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
)

// RequestParser 中间件：解析请求体并将数据存储到上下文中
// 避免在每个接口中重复解析请求数据
func RequestParser(r *ghttp.Request) {
	ctx := r.Context()
	method := r.Method

	// 只处理有请求体的方法
	if method != "POST" && method != "PUT" && method != "PATCH" {
		r.Middleware.Next()
		return
	}

	// 使用GoFrame的方式获取请求体内容
	bodyBytes := r.GetBody()
	if len(bodyBytes) == 0 {
		util.WriteBadRequest(r, "请求体不能为空")
		return
	}

	// 解析 JSON 请求体
	var reqData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		util.WriteBadRequest(r, "请求体必须是有效的JSON格式")
		return
	}

	// 将解析后的数据存储到上下文中
	ctx = context.WithValue(ctx, "requestData", reqData)
	r.SetCtx(ctx)

	r.Middleware.Next()
}

// GetRequestData 从上下文中获取已解析的请求数据
// 这是一个辅助函数，供其他地方使用
func GetRequestData(ctx context.Context) (map[string]interface{}, error) {
	if data := ctx.Value("requestData"); data != nil {
		if reqData, ok := data.(map[string]interface{}); ok {
			return reqData, nil
		}
		return nil, fmt.Errorf("invalid request data type")
	}
	return nil, fmt.Errorf("request data not found in context")
}

// GetRequestDataWithFallback 从上下文中获取请求数据，如果没有则尝试解析请求体
// 这个函数用于向后兼容，确保即使没有使用中间件也能正常工作
func GetRequestDataWithFallback(ctx context.Context, r *ghttp.Request) (map[string]interface{}, error) {
	// 首先尝试从上下文获取
	if data := ctx.Value("requestData"); data != nil {
		if reqData, ok := data.(map[string]interface{}); ok {
			return reqData, nil
		}
	}

	// 如果上下文中没有数据，尝试解析请求体
	bodyBytes := r.GetBody()
	if len(bodyBytes) == 0 {
		return nil, fmt.Errorf("请求体不能为空")
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		return nil, fmt.Errorf("请求体必须是有效的JSON格式: %v", err)
	}

	return reqData, nil
}
