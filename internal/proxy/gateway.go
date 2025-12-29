package proxy

import (
	"context"
	"fmt"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/registry"
	"net"

	"github.com/gogf/gf/v2/frame/g"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// StartGRPCGateway 启动 gRPC 反向代理网关
// 监听在指定端口，将所有 gRPC 请求转发到后端服务
func StartGRPCGateway(ctx context.Context, gatewayPort string) error {
	listener, err := net.Listen("tcp", gatewayPort)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", gatewayPort, err)
	}

	g.Log().Infof(ctx, "gRPC gateway listening on %s", gatewayPort)

	// 创建 gRPC 服务器，使用 StatsHandler
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(middleware.NewGatewayStatsHandler()),
		grpc.UnknownServiceHandler(grpcProxyHandler),
	)

	// 在 goroutine 中运行
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			g.Log().Errorf(ctx, "gRPC gateway server error: %v", err)
		}
	}()

	return nil
}

// grpcProxyHandler 处理所有未知的 gRPC 请求，转发到后端服务
func grpcProxyHandler(srv interface{}, serverStream grpc.ServerStream) error {
	// 从方法名中提取服务名
	// 例如：/user.UserService/GetUser -> admin_service
	serviceName := "admin_service" // 这里可以根据 fullMethod 动态确定

	// 从 Consul 获取服务地址
	addr, err := registry.GetServiceAddr(serviceName)
	if err != nil {
		return fmt.Errorf("service not available: %v", err)
	}

	// 连接到后端 gRPC 服务
	conn, err := grpc.DialContext(
		serverStream.Context(),
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to dial backend service: %v", err)
	}
	defer conn.Close()

	// 获取完整的方法名
	method, ok := grpc.Method(serverStream.Context())
	if !ok {
		method = "/unknown"
	}

	// 创建客户端流
	clientStream, err := conn.NewStream(
		serverStream.Context(),
		&grpc.StreamDesc{
			StreamName:    "proxy",
			ServerStreams: true,
			ClientStreams: true,
		},
		method,
	)
	if err != nil {
		return fmt.Errorf("failed to create client stream: %v", err)
	}

	// 双向转发数据
	go forwardClientToServer(serverStream, clientStream)
	forwardServerToClient(clientStream, serverStream)

	return nil
}

// forwardClientToServer 转发客户端数据到服务器
func forwardClientToServer(serverStream grpc.ServerStream, clientStream grpc.ClientStream) {
	for {
		frame := make([]byte, 1024*64)
		err := serverStream.RecvMsg(frame)
		if err != nil {
			clientStream.CloseSend()
			return
		}
		clientStream.SendMsg(frame)
	}
}

// forwardServerToClient 转发服务器数据到客户端
func forwardServerToClient(clientStream grpc.ClientStream, serverStream grpc.ServerStream) {
	for {
		frame := make([]byte, 1024*64)
		err := clientStream.RecvMsg(frame)
		if err != nil {
			return
		}
		serverStream.SendMsg(frame)
	}
}
