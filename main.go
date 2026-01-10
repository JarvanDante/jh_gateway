package main

import (
	"context"
	"fmt"
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
		fmt.Printf("加载时区失败: %v，使用默认时区\n", err)
		loc, _ = time.LoadLocation("Asia/Shanghai")
	}
	time.Local = loc
	fmt.Printf("时区设置: %s\n", timezone)

	// 强制输出到stdout确保能看到
	fmt.Println("=== jh_gateway 启动 ===")
	fmt.Printf("启动时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("进程ID: %d\n", os.Getpid())
	fmt.Println("========================")

	// 初始化Jaeger追踪
	cleanup, err := tracing.InitJaeger()
	if err != nil {
		fmt.Printf("初始化Jaeger失败: %v\n", err)
		g.Log().Errorf(ctx, "初始化Jaeger失败: %v", err)
	} else {
		fmt.Println("Jaeger追踪初始化成功")
		g.Log().Info(ctx, "Jaeger追踪初始化成功")
		defer cleanup()
	}

	// 初始化 Consul 客户端
	fmt.Println("初始化Consul客户端...")
	registry.InitConsul()

	// 启动 gRPC 网关（在后台运行）
	fmt.Println("启动gRPC网关...")
	grpcAddr := g.Cfg().MustGet(ctx, "grpcGateway.address").String()
	if err := proxy.StartGRPCGateway(ctx, grpcAddr); err != nil {
		g.Log().Fatalf(ctx, "failed to start gRPC gateway: %v", err)
	}

	// 启动 HTTP 网关
	fmt.Println("启动HTTP网关...")
	s := g.Server()
	router.Register(s)

	fmt.Println("网关服务启动完成")
	// 强制刷新输出
	os.Stdout.Sync()

	s.Run()
}
