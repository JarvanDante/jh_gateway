package util

import (
	"github.com/gogf/gf/v2/net/ghttp"
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
	WriteError(r, CodeInternalError, msg)
}

// WriteServiceUnavailable 503 错误
func WriteServiceUnavailable(r *ghttp.Request, msg string) {
	WriteError(r, CodeServiceUnavailable, msg)
}
