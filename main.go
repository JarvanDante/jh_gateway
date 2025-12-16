package main

import (
	"context"
	"jh_gateway/internal/proxy"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/router"

	"github.com/gogf/gf/v2/frame/g"
)

func main() {
	ctx := context.Background()

	// 初始化 Consul 客户端
	registry.InitConsul()

	// 启动 gRPC 网关（在后台运行）
	grpcAddr := g.Cfg().MustGet(ctx, "grpcGateway.address").String()
	if err := proxy.StartGRPCGateway(ctx, grpcAddr); err != nil {
		g.Log().Fatalf(ctx, "failed to start gRPC gateway: %v", err)
	}

	// 启动 HTTP 网关
	s := g.Server()
	router.Register(s)

	s.Run()
}
