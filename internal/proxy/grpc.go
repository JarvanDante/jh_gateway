package proxy

import (
	"context"
	"fmt"
	"jh_gateway/internal/registry"

	"github.com/gogf/gf/v2/frame/g"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCProxy gRPC 代理
type GRPCProxy struct {
	conns map[string]*grpc.ClientConn
}

var grpcProxy = &GRPCProxy{
	conns: make(map[string]*grpc.ClientConn),
}

// GetGRPCConn 获取或创建 gRPC 连接
func GetGRPCConn(ctx context.Context, serviceName string) (*grpc.ClientConn, error) {
	// 检查是否已有连接
	if conn, ok := grpcProxy.conns[serviceName]; ok && conn != nil {
		return conn, nil
	}

	// 从 Consul 获取服务地址
	addr, err := registry.GetServiceAddr(serviceName)
	if err != nil {
		return nil, fmt.Errorf("service not available: %v", err)
	}

	// 创建新连接
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*100)), // 100MB
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial grpc service: %v", err)
	}

	// 缓存连接
	grpcProxy.conns[serviceName] = conn

	g.Log().Infof(ctx, "grpc connection established for service: %s at %s", serviceName, addr)
	return conn, nil
}

// CloseGRPCConn 关闭 gRPC 连接
func CloseGRPCConn(serviceName string) error {
	if conn, ok := grpcProxy.conns[serviceName]; ok {
		delete(grpcProxy.conns, serviceName)
		return conn.Close()
	}
	return nil
}

// CloseAllGRPCConns 关闭所有 gRPC 连接
func CloseAllGRPCConns() error {
	for serviceName, conn := range grpcProxy.conns {
		if err := conn.Close(); err != nil {
			g.Log().Errorf(context.Background(), "failed to close grpc connection for %s: %v", serviceName, err)
		}
	}
	grpcProxy.conns = make(map[string]*grpc.ClientConn)
	return nil
}
