package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/util"
	"strconv"
	"strings"

	userv1 "jh_gateway/api/user/v1"

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
			util.WriteServiceUnavailable(r, "["+serviceName+"]：service not available")
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
	// 解析查询参数
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 10).Int32()

	g.Log().Infof(ctx, "calling gRPC GetList with page=%d, size=%d", page, size)

	// 创建 gRPC 客户端
	client := userv1.NewUserClient(conn)
	req := &userv1.GetListReq{
		Page: page,
		Size: size,
	}

	// 调用 gRPC 服务
	res, err := client.GetList(ctx, req)
	if err != nil {
		return fmt.Errorf("grpc call GetList failed: %v", err)
	}

	// 转换响应数据
	users := make([]map[string]interface{}, 0)
	for _, user := range res.Users {
		users = append(users, map[string]interface{}{
			"id":       user.Id,
			"passport": user.Passport,
			"nickname": user.Nickname,
			"createAt": user.CreateAt,
			"updateAt": user.UpdateAt,
		})
	}

	util.WriteSuccess(r, users)
	return nil
}

func callGetOne(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 从路径中提取 ID
	pathParts := strings.Split(r.URL.Path, "/")
	idStr := pathParts[len(pathParts)-1]

	// 转换 ID 为 uint64
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user id: %s", idStr)
	}

	g.Log().Infof(ctx, "calling gRPC GetOne with id=%d", id)

	// 创建 gRPC 客户端
	client := userv1.NewUserClient(conn)
	req := &userv1.GetOneReq{
		Id: id,
	}

	// 调用 gRPC 服务
	res, err := client.GetOne(ctx, req)
	if err != nil {
		return fmt.Errorf("grpc call GetOne failed: %v", err)
	}

	// 转换响应数据
	var userData map[string]interface{}
	if res.User != nil {
		userData = map[string]interface{}{
			"id":       res.User.Id,
			"passport": res.User.Passport,
			"nickname": res.User.Nickname,
			"createAt": res.User.CreateAt,
			"updateAt": res.User.UpdateAt,
		}
	}

	util.WriteSuccess(r, userData)
	return nil
}

func callCreate(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 解析 JSON 请求体
	var reqData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		return fmt.Errorf("invalid request body: %v", err)
	}

	// 提取必要字段
	passport, ok := reqData["passport"].(string)
	if !ok {
		return fmt.Errorf("passport is required")
	}
	password, ok := reqData["password"].(string)
	if !ok {
		return fmt.Errorf("password is required")
	}
	nickname, ok := reqData["nickname"].(string)
	if !ok {
		return fmt.Errorf("nickname is required")
	}

	g.Log().Infof(ctx, "calling gRPC Create with passport=%s, nickname=%s", passport, nickname)

	// 创建 gRPC 客户端
	client := userv1.NewUserClient(conn)
	req := &userv1.CreateReq{
		Passport: passport,
		Password: password,
		Nickname: nickname,
	}

	// 调用 gRPC 服务
	_, err := client.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("grpc call Create failed: %v", err)
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": "User created successfully",
	})
	return nil
}

func callDelete(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 从路径中提取 ID
	pathParts := strings.Split(r.URL.Path, "/")
	idStr := pathParts[len(pathParts)-1]

	// 转换 ID 为 uint64
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user id: %s", idStr)
	}

	g.Log().Infof(ctx, "calling gRPC Delete with id=%d", id)

	// 创建 gRPC 客户端
	client := userv1.NewUserClient(conn)
	req := &userv1.DeleteReq{
		Id: id,
	}

	// 调用 gRPC 服务
	_, err = client.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("grpc call Delete failed: %v", err)
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": "User deleted successfully",
		"id":      id,
	})
	return nil
}
