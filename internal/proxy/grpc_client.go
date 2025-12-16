package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/util"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCToHTTP 将 HTTP 请求转换为 gRPC 调用
func GRPCToHTTP(serviceName string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()

		// 从 Consul 获取服务地址
		addr, err := registry.GetServiceAddr(serviceName)
		if err != nil {
			util.WriteServiceUnavailable(r, "service not available")
			return
		}

		// 连接到 gRPC 服务
		conn, err := grpc.DialContext(
			ctx,
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			util.WriteServiceUnavailable(r, "failed to connect to service")
			return
		}
		defer conn.Close()

		// 根据路径调用不同的 gRPC 方法
		if err := callGRPCMethod(ctx, conn, r); err != nil {
			util.WriteInternalError(r, err.Error())
			return
		}
	}
}

// callGRPCMethod 根据 HTTP 路径调用对应的 gRPC 方法
func callGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	path := r.URL.Path
	method := r.Method

	g.Log().Infof(ctx, "calling gRPC method for path: %s, method: %s", path, method)

	// 根据路径映射到 gRPC 方法
	switch {
	case strings.HasSuffix(path, "/list") && method == "GET":
		return callGetList(ctx, conn, r)
	case strings.Contains(path, "/detail/") && method == "GET":
		return callGetOne(ctx, conn, r)
	case strings.HasSuffix(path, "/create") && method == "POST":
		return callCreate(ctx, conn, r)
	case strings.Contains(path, "/delete/") && method == "DELETE":
		return callDelete(ctx, conn, r)
	default:
		return fmt.Errorf("unsupported path: %s", path)
	}
}

// 这里需要根据实际的 proto 定义来实现具体的调用
// 由于没有生成的 gRPC 客户端代码，这里用通用的方式调用

func callGetList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 模拟调用 gRPC GetList 方法
	// 实际应该用生成的客户端代码

	// 模拟返回数据
	mockData := []map[string]interface{}{
		{"id": 1, "name": "Alice", "email": "alice@example.com"},
		{"id": 2, "name": "Bob", "email": "bob@example.com"},
		{"id": 3, "name": "Charlie", "email": "charlie@example.com"},
	}

	util.WriteSuccess(r, mockData)
	return nil
}

func callGetOne(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 从路径中提取 ID
	pathParts := strings.Split(r.URL.Path, "/")
	id := pathParts[len(pathParts)-1]

	// 模拟返回数据
	mockData := map[string]interface{}{
		"id":    id,
		"name":  "User " + id,
		"email": "user" + id + "@example.com",
	}

	util.WriteSuccess(r, mockData)
	return nil
}

func callCreate(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 解析请求体
	var reqData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		return fmt.Errorf("invalid request body: %v", err)
	}

	// 模拟创建用户
	mockData := map[string]interface{}{
		"id":    999,
		"name":  reqData["name"],
		"email": reqData["email"],
	}

	util.WriteSuccess(r, mockData)
	return nil
}

func callDelete(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 从路径中提取 ID
	pathParts := strings.Split(r.URL.Path, "/")
	id := pathParts[len(pathParts)-1]

	// 模拟删除
	mockData := map[string]interface{}{
		"id": id,
	}

	util.WriteSuccess(r, mockData)
	return nil
}
