package middleware

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/stats"
)

// GatewayStatsHandler 实现 gRPC stats.Handler 接口，用于网关的追踪和统计
type GatewayStatsHandler struct{}

// NewGatewayStatsHandler 创建新的 GatewayStatsHandler
func NewGatewayStatsHandler() *GatewayStatsHandler {
	return &GatewayStatsHandler{}
}

// TagRPC 在 RPC 开始时调用，可以返回一个 context 用于后续的统计调用
func (h *GatewayStatsHandler) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	// 记录 RPC 开始信息
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		g.Log().Debugf(ctx, "网关RPC开始: %s, TraceID: %s", info.FullMethodName, traceID)
	}

	// 在 context 中存储开始时间，用于计算耗时
	return context.WithValue(ctx, "gateway_rpc_start_time", time.Now())
}

// HandleRPC 处理 RPC 统计信息
func (h *GatewayStatsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	switch stat := s.(type) {
	case *stats.Begin:
		// RPC 开始
		h.handleRPCBegin(ctx, stat)
	case *stats.InPayload:
		// 接收到请求数据
		h.handleInPayload(ctx, stat)
	case *stats.OutPayload:
		// 发送响应数据
		h.handleOutPayload(ctx, stat)
	case *stats.End:
		// RPC 结束
		h.handleRPCEnd(ctx, stat)
	}
}

// TagConn 在连接建立时调用
func (h *GatewayStatsHandler) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	g.Log().Debugf(ctx, "网关新连接建立: %s -> %s", info.RemoteAddr, info.LocalAddr)
	return ctx
}

// HandleConn 处理连接统计信息
func (h *GatewayStatsHandler) HandleConn(ctx context.Context, s stats.ConnStats) {
	switch stat := s.(type) {
	case *stats.ConnBegin:
		g.Log().Debugf(ctx, "网关连接开始")
	case *stats.ConnEnd:
		g.Log().Debugf(ctx, "网关连接结束")
	default:
		g.Log().Debugf(ctx, "网关连接统计: %T", stat)
	}
}

// handleRPCBegin 处理 RPC 开始事件
func (h *GatewayStatsHandler) handleRPCBegin(ctx context.Context, stat *stats.Begin) {
	traceID := getTraceIDFromContext(ctx)

	logData := g.Map{
		"service":    "gateway",
		"trace_id":   traceID,
		"event":      "rpc_begin",
		"client":     stat.Client,
		"begin_time": stat.BeginTime.Format(time.RFC3339Nano),
	}

	g.Log().Debug(ctx, logData)
}

// handleInPayload 处理接收请求数据事件
func (h *GatewayStatsHandler) handleInPayload(ctx context.Context, stat *stats.InPayload) {
	traceID := getTraceIDFromContext(ctx)

	logData := g.Map{
		"service":        "gateway",
		"trace_id":       traceID,
		"event":          "in_payload",
		"payload_length": stat.Length,
		"wire_length":    stat.WireLength,
		"received_time":  stat.RecvTime.Format(time.RFC3339Nano),
	}

	g.Log().Debug(ctx, logData)
}

// handleOutPayload 处理发送响应数据事件
func (h *GatewayStatsHandler) handleOutPayload(ctx context.Context, stat *stats.OutPayload) {
	traceID := getTraceIDFromContext(ctx)

	logData := g.Map{
		"service":        "gateway",
		"trace_id":       traceID,
		"event":          "out_payload",
		"payload_length": stat.Length,
		"wire_length":    stat.WireLength,
		"sent_time":      stat.SentTime.Format(time.RFC3339Nano),
	}

	g.Log().Debug(ctx, logData)
}

// handleRPCEnd 处理 RPC 结束事件
func (h *GatewayStatsHandler) handleRPCEnd(ctx context.Context, stat *stats.End) {
	traceID := getTraceIDFromContext(ctx)

	// 计算总耗时
	var duration time.Duration
	if startTime, ok := ctx.Value("gateway_rpc_start_time").(time.Time); ok {
		duration = stat.EndTime.Sub(startTime)
	}

	logData := g.Map{
		"service":  "gateway",
		"trace_id": traceID,
		"event":    "rpc_end",
		"end_time": stat.EndTime.Format(time.RFC3339Nano),
		"duration": duration.String(),
		"client":   stat.Client,
	}

	// 如果有错误，记录错误信息
	if stat.Error != nil {
		logData["error"] = stat.Error.Error()
		g.Log().Error(ctx, logData)
	} else {
		g.Log().Info(ctx, logData)
	}
}

// getTraceIDFromContext 从上下文中获取OpenTelemetry的TraceID
func getTraceIDFromContext(ctx context.Context) string {
	// 只使用OpenTelemetry的TraceID
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}
