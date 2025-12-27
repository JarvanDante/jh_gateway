package util

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel/trace"
)

// LogWithTrace 网关带traceId的日志记录 - JSON格式
func LogWithTrace(ctx context.Context, level string, message string, args ...interface{}) {
	// 从上下文中获取traceId
	traceID := getTraceIDFromRequest(ctx)

	// 构建结构化日志数据
	logData := g.Map{
		"service":  "gateway",
		"trace_id": traceID,
		"msg":      fmt.Sprintf(message, args...),
	}

	// 根据级别记录日志
	switch strings.ToLower(level) {
	case "debug":
		g.Log().Debug(ctx, logData)
	case "info":
		g.Log().Info(ctx, logData)
	case "warn", "warning":
		g.Log().Warning(ctx, logData)
	case "error":
		g.Log().Error(ctx, logData)
	case "fatal":
		g.Log().Fatal(ctx, logData)
	default:
		g.Log().Info(ctx, logData)
	}
}

// getTraceIDFromRequest 从请求中获取OpenTelemetry的TraceID
func getTraceIDFromRequest(ctx context.Context) string {
	// 只使用OpenTelemetry的TraceID
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}

	return ""
}
