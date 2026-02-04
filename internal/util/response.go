package util

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc/status"
)

// Response 统一响应结构
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// Error codes
const (
	CodeSuccess            = 0
	CodeBadRequest         = 400
	CodeUnauthorized       = 401
	CodeForbidden          = 403
	CodeNotFound           = 404
	CodeTooManyRequests    = 429
	CodeInternalError      = 500
	CodeServiceUnavailable = 503
)

// ErrorMessages 错误消息映射
var ErrorMessages = map[int]string{
	CodeSuccess:            "success",
	CodeBadRequest:         "bad request",
	CodeUnauthorized:       "unauthorized",
	CodeForbidden:          "forbidden",
	CodeNotFound:           "not found",
	CodeTooManyRequests:    "too many requests",
	CodeInternalError:      "internal server error",
	CodeServiceUnavailable: "service unavailable",
}

// ExtractGRPCErrorMessage 从gRPC错误中提取纯净的错误消息
// 将 "rpc error: code = Unknown desc = 已经绑定过密码请勿重新修改" 转换为 "已经绑定过密码请勿重新修改"
func ExtractGRPCErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	// 尝试使用grpc status提取错误消息
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}

	// 如果不是标准gRPC错误，尝试手动解析
	errStr := err.Error()

	// 处理 "rpc error: code = XXX desc = YYY" 格式
	if strings.Contains(errStr, "desc = ") {
		parts := strings.SplitN(errStr, "desc = ", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}

	return errStr
}

// WriteSuccess 返回成功响应
func WriteSuccess(r *ghttp.Request, data interface{}) {
	r.Response.WriteJsonExit(Response{
		Code: CodeSuccess,
		Msg:  ErrorMessages[CodeSuccess],
		Data: data,
	})
}

// WriteError 返回错误响应
func WriteError(r *ghttp.Request, code int, msg string) {
	if msg == "" {
		msg = ErrorMessages[code]
	}
	msg = cleanGRPCErrorMessage(msg)
	// 不设置HTTP状态码，始终返回200，错误信息在JSON中体现
	r.Response.WriteJsonExit(Response{
		Code: code,
		Msg:  msg,
	})
}

// WriteBadRequest 400 错误
func WriteBadRequest(r *ghttp.Request, msg string) {
	WriteError(r, CodeBadRequest, msg)
}

// WriteUnauthorized 401 错误
func WriteUnauthorized(r *ghttp.Request, msg string) {
	WriteError(r, CodeUnauthorized, msg)
}

// WriteForbidden 403 错误
func WriteForbidden(r *ghttp.Request, msg string) {
	WriteError(r, CodeForbidden, msg)
}

// WriteNotFound 404 错误
func WriteNotFound(r *ghttp.Request, msg string) {
	WriteError(r, CodeNotFound, msg)
}

// WriteTooManyRequests 429 错误
func WriteTooManyRequests(r *ghttp.Request, msg string) {
	WriteError(r, CodeTooManyRequests, msg)
}

// WriteInternalError 500 错误
func WriteInternalError(r *ghttp.Request, msg string) {
	// 清理gRPC错误前缀
	cleanMsg := cleanGRPCErrorMessage(msg)
	WriteError(r, CodeInternalError, cleanMsg)
}

// NormalizeMessage 清理gRPC错误消息中的前缀
func NormalizeMessage(msg string) string {
	return cleanGRPCErrorMessage(msg)
}

// cleanGRPCErrorMessage 清理gRPC错误消息中的前缀
func cleanGRPCErrorMessage(msg string) string {
	// 处理 "rpc error: code = XXX desc = YYY" 格式
	if strings.Contains(msg, "desc = ") {
		parts := strings.SplitN(msg, "desc = ", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return msg
}

// WriteServiceUnavailable 503 错误
func WriteServiceUnavailable(r *ghttp.Request, msg string) {
	WriteError(r, CodeServiceUnavailable, msg)
}
