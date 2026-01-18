package main

import (
	"context"
	"os"
	"time"

	"jh_gateway/internal/proxy"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/router"
	"jh_gateway/internal/tracing"

	"github.com/gogf/gf/v2/frame/g"
)

func main() {
	// 在最早的时刻设置时区（在创建context之前）
	ctx := context.Background()

	// 需要先创建context来读取配置，所以在这里设置
	timezone := g.Cfg().MustGet(ctx, "timezone").String()
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		g.Log().Errorf(ctx, "加载时区失败: %v，使用默认时区", err)
		loc, _ = time.LoadLocation("Asia/Shanghai")
	}
	time.Local = loc
	g.Log().Infof(ctx, "时区设置: %s", timezone)

	// 启动日志
	g.Log().Info(ctx, "=== jh_gateway 启动 ===")
	g.Log().Infof(ctx, "启动时间: %s", time.Now().Format("2006-01-02 15:04:05"))
	g.Log().Infof(ctx, "进程ID: %d", os.Getpid())
	g.Log().Info(ctx, "========================")

	// 初始化Jaeger追踪
	cleanup, err := tracing.InitJaeger()
	if err != nil {
		g.Log().Errorf(ctx, "初始化Jaeger失败: %v", err)
	} else {
		g.Log().Info(ctx, "Jaeger追踪初始化成功")
		defer cleanup()
	}

	// 初始化 Consul 客户端
	g.Log().Info(ctx, "初始化Consul客户端...")
	registry.InitConsul()

	// 初始化域名监听器
	g.Log().Info(ctx, "初始化域名监听器...")
	if err := registry.InitDomainWatcher(); err != nil {
		g.Log().Errorf(ctx, "初始化域名监听器失败: %v", err)
	} else {
		g.Log().Info(ctx, "域名监听器初始化成功")
	}

	// 检查域名配置加载状态
	healthStatus := registry.GetHealthStatus()
	g.Log().Infof(ctx, "域名配置状态: %v", healthStatus)

	domainCount := registry.GetDomainCount()
	siteCount := registry.GetSiteCount()

	if domainCount == 0 {
		g.Log().Warning(ctx, "⚠️  警告：未加载任何域名配置！")
		g.Log().Warning(ctx, "⚠️  请检查Consul KV配置: sites/")
		g.Log().Warning(ctx, "⚠️  Gateway将继续运行，但域名解析将失败")
	} else {
		g.Log().Infof(ctx, "✅ 域名配置加载成功: %d个域名映射到%d个站点", domainCount, siteCount)
	}

	// 启动 gRPC 网关（在后台运行）
	g.Log().Info(ctx, "启动gRPC网关...")
	grpcAddr := g.Cfg().MustGet(ctx, "grpcGateway.address").String()
	if err := proxy.StartGRPCGateway(ctx, grpcAddr); err != nil {
		g.Log().Fatalf(ctx, "failed to start gRPC gateway: %v", err)
	}

	// 启动 HTTP 网关
	g.Log().Info(ctx, "启动HTTP网关...")
	s := g.Server()
	router.Register(s)

	g.Log().Info(ctx, "网关服务启动完成")

	s.Run()
}
