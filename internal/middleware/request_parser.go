package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"jh_gateway/internal/util"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// RequestParser 中间件：解析请求参数并将数据存储到上下文中
// 支持 GET 查询参数、POST JSON、POST form-data 等多种格式
func RequestParser(r *ghttp.Request) {
	ctx := r.Context()
	method := r.Method
	path := r.URL.Path
	contentType := r.Header.Get("Content-Type")

	var reqData map[string]interface{}

	// 处理 GET 请求的查询参数
	if method == "GET" {
		reqData = make(map[string]interface{})

		// 获取所有查询参数 - GoFrame 会自动解析 URL 查询参数
		for key, value := range r.GetQueryMap() {
			reqData[key] = value
		}

		util.LogWithTrace(ctx, "debug", "解析GET查询参数: %+v", reqData)

		// 将解析后的数据存储到上下文中
		ctx = context.WithValue(ctx, "requestData", reqData)
		r.SetCtx(ctx)
		r.Middleware.Next()
		return
	}

	// 处理 POST、PUT、PATCH 请求
	if method != "POST" && method != "PUT" && method != "PATCH" {
		// 其他方法存储空map
		ctx = context.WithValue(ctx, "requestData", map[string]interface{}{})
		r.SetCtx(ctx)
		r.Middleware.Next()
		return
	}

	// 跳过文件上传接口，因为它们需要特殊处理文件
	if isFileUploadEndpoint(path) {
		r.Middleware.Next()
		return
	}

	// 处理 application/x-www-form-urlencoded 格式
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		reqData = make(map[string]interface{})

		// 将表单数据转换为map - GoFrame 会自动解析表单数据
		for key, value := range r.GetFormMap() {
			reqData[key] = value
		}

		util.LogWithTrace(ctx, "debug", "解析POST表单数据(urlencoded): %+v", reqData)

		// 将解析后的数据存储到上下文中
		ctx = context.WithValue(ctx, "requestData", reqData)
		r.SetCtx(ctx)
		r.Middleware.Next()
		return
	}

	// 处理 multipart/form-data 格式（非文件上传）
	if strings.HasPrefix(contentType, "multipart/form-data") {
		reqData = make(map[string]interface{})

		// 解析 multipart 表单数据
		if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB 最大内存
			util.LogWithTrace(ctx, "warn", "解析multipart表单失败: %v", err)
			// 如果解析失败，尝试使用 GoFrame 的方法
			for key, value := range r.GetFormMap() {
				reqData[key] = value
			}
		} else {
			// 成功解析后，提取表单字段（非文件字段）
			if r.MultipartForm != nil && r.MultipartForm.Value != nil {
				for key, values := range r.MultipartForm.Value {
					if len(values) > 0 {
						if len(values) == 1 {
							reqData[key] = values[0]
						} else {
							reqData[key] = values
						}
					}
				}
			}
		}

		util.LogWithTrace(ctx, "debug", "解析POST表单数据(multipart): %+v", reqData)

		// 将解析后的数据存储到上下文中
		ctx = context.WithValue(ctx, "requestData", reqData)
		r.SetCtx(ctx)
		r.Middleware.Next()
		return
	}

	// 处理 JSON 格式请求体
	bodyBytes := r.GetBody()

	// 如果请求体为空，存储空的map到上下文中
	if len(bodyBytes) == 0 {
		ctx = context.WithValue(ctx, "requestData", map[string]interface{}{})
		r.SetCtx(ctx)
		r.Middleware.Next()
		return
	}

	// 解析 JSON 请求体
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		util.WriteBadRequest(r, "请求体必须是有效的JSON格式")
		return
	}

	util.LogWithTrace(ctx, "debug", "解析JSON请求体: %+v", reqData)

	// 将解析后的数据存储到上下文中
	ctx = context.WithValue(ctx, "requestData", reqData)
	r.SetCtx(ctx)

	r.Middleware.Next()
}

// isFileUploadEndpoint 判断是否为文件上传接口
func isFileUploadEndpoint(path string) bool {
	// 文件上传相关的接口路径
	uploadPaths := []string{
		"/upload-image",
		"/upload-file",
		"/upload",
	}

	for _, uploadPath := range uploadPaths {
		if strings.HasSuffix(path, uploadPath) {
			return true
		}
	}

	return false
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

// GetRequestDataWithFallback 从上下文中获取请求数据，如果没有则尝试解析请求体或查询参数
// 这个函数用于向后兼容，确保即使没有使用中间件也能正常工作
// allowEmpty: 是否允许空请求体
func GetRequestDataWithFallback(ctx context.Context, r *ghttp.Request) (map[string]interface{}, error) {
	return GetRequestDataWithOptions(ctx, r, false)
}

// GetRequestDataWithOptions 从上下文中获取请求数据，支持更多选项
// allowEmpty: 是否允许空请求体
func GetRequestDataWithOptions(ctx context.Context, r *ghttp.Request, allowEmpty bool) (map[string]interface{}, error) {
	// 首先尝试从上下文获取
	if data := ctx.Value("requestData"); data != nil {
		if reqData, ok := data.(map[string]interface{}); ok {
			// 如果不允许空请求体且数据为空，返回错误
			if !allowEmpty && len(reqData) == 0 {
				return nil, fmt.Errorf("请求体不能为空")
			}
			return reqData, nil
		}
	}

	// 如果上下文中没有数据，尝试手动解析
	method := r.Method
	contentType := r.Header.Get("Content-Type")

	// 处理 GET 请求的查询参数
	if method == "GET" {
		reqData := make(map[string]interface{})
		for key, value := range r.GetQueryMap() {
			reqData[key] = value
		}

		if !allowEmpty && len(reqData) == 0 {
			return nil, fmt.Errorf("请求参数不能为空")
		}
		return reqData, nil
	}

	// 处理 POST 表单数据
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		reqData := make(map[string]interface{})

		// 将表单数据转换为map
		for key, value := range r.GetFormMap() {
			reqData[key] = value
		}

		if !allowEmpty && len(reqData) == 0 {
			return nil, fmt.Errorf("表单数据不能为空")
		}
		return reqData, nil
	}

	// 处理 multipart/form-data 格式（非文件上传）
	if strings.HasPrefix(contentType, "multipart/form-data") {
		reqData := make(map[string]interface{})

		// 解析 multipart 表单数据
		if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB 最大内存
			// 如果解析失败，尝试使用 GoFrame 的方法
			for key, value := range r.GetFormMap() {
				reqData[key] = value
			}
		} else {
			// 成功解析后，提取表单字段（非文件字段）
			if r.MultipartForm != nil && r.MultipartForm.Value != nil {
				for key, values := range r.MultipartForm.Value {
					if len(values) > 0 {
						if len(values) == 1 {
							reqData[key] = values[0]
						} else {
							reqData[key] = values
						}
					}
				}
			}
		}

		if !allowEmpty && len(reqData) == 0 {
			return nil, fmt.Errorf("multipart表单数据不能为空")
		}
		return reqData, nil
	}

	// 处理 JSON 请求体
	bodyBytes := r.GetBody()
	if len(bodyBytes) == 0 {
		if allowEmpty {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("请求体不能为空")
	}

	var reqData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqData); err != nil {
		return nil, fmt.Errorf("请求体必须是有效的JSON格式: %v", err)
	}

	return reqData, nil
}
