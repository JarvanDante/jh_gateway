package tracing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials/insecure"
)

var tracer trace.Tracer

// InitJaeger 初始化Jaeger追踪，从配置文件读取参数
func InitJaeger() (func(), error) {
	ctx := context.Background()

	// 从配置文件读取Jaeger配置
	enabled := g.Cfg().MustGet(ctx, "jaeger.enabled", true).Bool()
	if !enabled {
		fmt.Println("Jaeger追踪已禁用")
		return func() {}, nil
	}

	endpoint := g.Cfg().MustGet(ctx, "jaeger.endpoint").String()
	serviceName := g.Cfg().MustGet(ctx, "jaeger.serviceName").String()
	serviceVersion := g.Cfg().MustGet(ctx, "jaeger.serviceVersion", "1.0.0").String()
	environment := g.Cfg().MustGet(ctx, "jaeger.environment", "development").String()

	if endpoint == "" {
		return nil, fmt.Errorf("jaeger.endpoint 配置不能为空")
	}
	if serviceName == "" {
		return nil, fmt.Errorf("jaeger.serviceName 配置不能为空")
	}

	fmt.Printf("正在初始化Jaeger追踪: 服务名=%s, 端点=%s, 版本=%s, 环境=%s\n",
		serviceName, endpoint, serviceVersion, environment)

	// 创建OTLP gRPC导出器
	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()), // 开发环境使用不安全连接
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
	}

	fmt.Println("OTLP gRPC导出器创建成功")

	// 创建资源
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			attribute.String("environment", environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 创建追踪提供者
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxExportBatchSize(1),       // 立即导出，不等待批量
			sdktrace.WithBatchTimeout(1*time.Second), // 1秒超时
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // 开发环境采样所有请求
	)

	// 设置全局追踪提供者
	otel.SetTracerProvider(tp)

	// 设置全局传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 获取追踪器
	tracer = tp.Tracer(serviceName)

	fmt.Printf("Jaeger追踪初始化完成: 服务名=%s\n", serviceName)

	// 返回清理函数
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down tracer provider: %v\n", err)
		}
	}

	return cleanup, nil
}

// InitJaegerWithParams 使用参数初始化Jaeger追踪（保持向后兼容）
func InitJaegerWithParams(serviceName, jaegerEndpoint string) (func(), error) {
	fmt.Printf("正在初始化Jaeger追踪: 服务名=%s, 端点=%s\n", serviceName, jaegerEndpoint)

	// 创建OTLP gRPC导出器
	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(jaegerEndpoint),
		otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()), // 开发环境使用不安全连接
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
	}

	fmt.Println("OTLP gRPC导出器创建成功")

	// 创建资源
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("environment", "development"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 创建追踪提供者
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithMaxExportBatchSize(1),       // 立即导出，不等待批量
			sdktrace.WithBatchTimeout(1*time.Second), // 1秒超时
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // 开发环境采样所有请求
	)

	// 设置全局追踪提供者
	otel.SetTracerProvider(tp)

	// 设置全局传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 获取追踪器
	tracer = tp.Tracer(serviceName)

	fmt.Printf("Jaeger追踪初始化完成: 服务名=%s\n", serviceName)

	// 返回清理函数
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down tracer provider: %v\n", err)
		}
	}

	return cleanup, nil
}

// GetTracer 获取追踪器
func GetTracer() trace.Tracer {
	return tracer
}

// StartSpan 开始一个新的Span
func StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tracer.Start(ctx, spanName, opts...)
}

// AddSpanEvent 添加Span事件
func AddSpanEvent(span trace.Span, name string, attributes ...attribute.KeyValue) {
	span.AddEvent(name, trace.WithAttributes(attributes...))
}

// SetSpanError 设置Span错误
func SetSpanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("error", true))
	}
}

// SetSpanAttributes 设置Span属性
func SetSpanAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}
