package proxy

import (
	"context"
	"fmt"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/tracing"
	"jh_gateway/internal/util"
	"strconv"
	"strings"

	adminv1 "jh_gateway/api/admin/v1"
	userv1 "jh_gateway/api/user/v1"

	"github.com/gogf/gf/v2/net/ghttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCToHTTP 将 HTTP 请求转换为 gRPC 调用
func GRPCToHTTP(serviceName string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		ctx := r.Context()
		path := r.URL.Path
		method := r.Method

		// 创建HTTP请求的Jaeger span
		ctx, span := tracing.StartSpan(ctx, "http.request", trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("service.name", serviceName),
		))
		defer span.End()

		// 先进行参数验证，避免无效请求浪费资源
		var requestData map[string]interface{}
		if shouldValidateEarly(path, method) {
			ctx, validateSpan := tracing.StartSpan(ctx, "http.validate_request")
			var err error
			requestData, err = validateAndParseRequest(ctx, r)
			validateSpan.End()

			if err != nil {
				tracing.SetSpanError(span, err)
				tracing.SetSpanError(validateSpan, err)
				// 验证失败，已经写入响应，直接返回
				return
			}
			// 将解析后的数据存储到上下文中，供后续使用
			ctx = context.WithValue(ctx, "requestData", requestData)
		}

		// 从 Consul 获取服务地址（支持负载均衡）
		ctx, discoverySpan := tracing.StartSpan(ctx, "service.discovery", trace.WithAttributes(
			attribute.String("service.name", serviceName),
		))
		addr, err := registry.GetServiceAddr(serviceName)
		discoverySpan.End()

		if err != nil {
			tracing.SetSpanError(span, err)
			tracing.SetSpanError(discoverySpan, err)
			util.WriteServiceUnavailable(r, "["+serviceName+"]：service not available")
			return
		}

		tracing.SetSpanAttributes(span, attribute.String("service.address", addr))
		// 记录选择的服务实例
		util.LogWithTrace(ctx, "info", "Selected service instance: %s -> %s", serviceName, addr)

		// 连接到 gRPC 服务
		ctx, connSpan := tracing.StartSpan(ctx, "grpc.connect", trace.WithAttributes(
			attribute.String("grpc.target", addr),
		))
		conn, err := grpc.DialContext(
			ctx,
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()), // 添加OpenTelemetry gRPC拦截器
		)
		connSpan.End()

		if err != nil {
			tracing.SetSpanError(span, err)
			tracing.SetSpanError(connSpan, err)
			util.WriteServiceUnavailable(r, "failed to connect to service")
			return
		}
		defer conn.Close()

		// 根据路径调用不同的 gRPC 方法
		if err := callGRPCMethod(ctx, conn, r); err != nil {
			tracing.SetSpanError(span, err)
			util.WriteInternalError(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
	}
}

// callGRPCMethod 根据 HTTP 路径调用对应的 gRPC 方法
func callGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	path := r.URL.Path
	method := r.Method

	// 统一添加traceId到gRPC上下文 - 只需要在这里添加一次
	ctx = addTraceToContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling gRPC method for path: %s, method: %s", path, method)

	// 管理员相关接口
	switch {
	case strings.HasSuffix(path, "/login") && method == "POST":
		return callAdminLogin(ctx, conn, r)
	case strings.HasSuffix(path, "/refresh-token") && method == "GET":
		return callAdminRefreshToken(ctx, conn, r)
	case strings.HasSuffix(path, "/create-admin") && method == "POST":
		return callAdminCreate(ctx, conn, r)
	}

	// 用户相关接口
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

	util.LogWithTrace(ctx, "info", "calling gRPC GetList with page=%d, size=%d", page, size)

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

	util.LogWithTrace(ctx, "info", "calling gRPC GetOne with id=%d", id)

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
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, err.Error())
		return nil
	}

	// 提取必要字段
	passport, ok := reqData["passport"].(string)
	if !ok {
		util.WriteBadRequest(r, "passport is required")
		return nil
	}
	password, ok := reqData["password"].(string)
	if !ok {
		util.WriteBadRequest(r, "password is required")
		return nil
	}
	nickname, ok := reqData["nickname"].(string)
	if !ok {
		util.WriteBadRequest(r, "nickname is required")
		return nil
	}

	util.LogWithTrace(ctx, "info", "calling gRPC Create with passport=%s, nickname=%s", passport, nickname)

	// 创建 gRPC 客户端
	client := userv1.NewUserClient(conn)
	req := &userv1.CreateReq{
		Passport: passport,
		Password: password,
		Nickname: nickname,
	}

	// 调用 gRPC 服务
	_, err = client.Create(ctx, req)
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

	util.LogWithTrace(ctx, "info", "calling gRPC Delete with id=%d", id)

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

// Admin相关的gRPC调用函数

// callAdminLogin 处理管理员登录
func callAdminLogin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, err.Error())
		return nil
	}

	// 进行基本验证
	if err := validateLoginRequest(r, reqData); err != nil {
		return nil
	}

	// 提取字段
	username := reqData["username"].(string)
	password := reqData["password"].(string)
	code, _ := reqData["code"].(string)

	util.LogWithTrace(ctx, "info", "calling gRPC Admin Login with username=%s", username)

	// 创建 gRPC 客户端
	client := adminv1.NewAdminClient(conn)
	req := &adminv1.LoginReq{
		Username: username,
		Password: password,
		Code:     code,
	}

	// 调用 gRPC 服务
	res, err := client.Login(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "用户名或密码错误") {
			util.WriteForbidden(r, "用户名或密码错误")
		} else if strings.Contains(err.Error(), "账号已被禁用") {
			util.WriteForbidden(r, "账号已被禁用")
		} else {
			util.WriteInternalError(r, "登录服务暂时不可用，请稍后重试")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"token":  res.Token,
		"socket": res.Socket,
	})
	return nil
}

// callAdminRefreshToken 处理管理员token刷新
func callAdminRefreshToken(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin RefreshToken")

	// 创建 gRPC 客户端
	client := adminv1.NewAdminClient(conn)
	req := &adminv1.RefreshTokenReq{}

	// 调用 gRPC 服务
	res, err := client.RefreshToken(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "刷新token失败，请重新登录")
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"token": res.Token,
	})
	return nil
}

// callAdminCreate 处理创建管理员
func callAdminCreate(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, err.Error())
		return nil
	}

	// 进行基本验证
	if err := validateCreateAdminRequest(r, reqData); err != nil {
		return nil
	}

	// 提取字段
	username := reqData["username"].(string)
	password := reqData["password"].(string)
	nickname := reqData["nickname"].(string)

	// 角色和状态字段
	role := int32(1) // 默认角色
	if r, ok := reqData["role"].(float64); ok {
		role = int32(r)
	}

	status := int32(1) // 默认启用
	if s, ok := reqData["status"].(float64); ok {
		status = int32(s)
	}

	util.LogWithTrace(ctx, "info", "calling gRPC Admin CreateAdmin with username=%s, nickname=%s", username, nickname)

	// 创建 gRPC 客户端
	client := adminv1.NewAdminClient(conn)
	req := &adminv1.CreateAdminReq{
		Username: username,
		Password: password,
		Nickname: nickname,
		Role:     role,
		Status:   status,
	}

	// 调用 gRPC 服务
	_, err = client.CreateAdmin(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "用户名已经被使用") {
			util.WriteBadRequest(r, "用户名已经被使用")
		} else if strings.Contains(err.Error(), "数据库") {
			util.WriteInternalError(r, "数据库操作失败，请稍后重试")
		} else {
			util.WriteInternalError(r, "创建管理员失败，请稍后重试")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": "管理员创建成功",
	})
	return nil
}

// shouldValidateEarly 判断是否需要提前验证参数
func shouldValidateEarly(path, method string) bool {
	// 对于admin相关的POST请求，提前验证
	return (strings.HasSuffix(path, "/login") && method == "POST") ||
		(strings.HasSuffix(path, "/create-admin") && method == "POST")
}

// addTraceToContext 添加OpenTelemetry trace context到gRPC上下文
func addTraceToContext(ctx context.Context, r *ghttp.Request) context.Context {
	// OpenTelemetry会自动处理trace context的传播
	// 我们不需要手动传递trace_id，只需要确保使用正确的context

	// 记录当前的TraceID（仅用于调试）
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		util.LogWithTrace(ctx, "debug", "使用OpenTelemetry TraceID: %s", traceID)
	}

	// 直接返回context，让OpenTelemetry自动处理trace传播
	return ctx
}

// validateAndParseRequest 验证并解析请求参数
// 注意：这个函数现在主要用于向后兼容，推荐使用 RequestParser 中间件
func validateAndParseRequest(ctx context.Context, r *ghttp.Request) (map[string]interface{}, error) {
	// 使用中间件的辅助函数获取请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, err.Error())
		return nil, err
	}

	path := r.URL.Path
	method := r.Method

	// 根据不同接口验证不同参数
	if strings.HasSuffix(path, "/login") && method == "POST" {
		if err := validateLoginRequest(r, reqData); err != nil {
			return nil, err
		}
	}

	if strings.HasSuffix(path, "/create-admin") && method == "POST" {
		if err := validateCreateAdminRequest(r, reqData); err != nil {
			return nil, err
		}
	}

	return reqData, nil
}

// validateLoginRequest 验证登录请求参数
func validateLoginRequest(r *ghttp.Request, reqData map[string]interface{}) error {
	username, ok := reqData["username"].(string)
	if !ok || username == "" {
		util.WriteBadRequest(r, "用户名不能为空")
		return fmt.Errorf("missing username")
	}

	password, ok := reqData["password"].(string)
	if !ok || password == "" {
		util.WriteBadRequest(r, "密码不能为空")
		return fmt.Errorf("missing password")
	}

	return nil
}

// validateCreateAdminRequest 验证创建管理员请求参数
func validateCreateAdminRequest(r *ghttp.Request, reqData map[string]interface{}) error {
	username, ok := reqData["username"].(string)
	if !ok || username == "" {
		util.WriteBadRequest(r, "用户名不能为空")
		return fmt.Errorf("missing username")
	}

	password, ok := reqData["password"].(string)
	if !ok || password == "" {
		util.WriteBadRequest(r, "密码不能为空")
		return fmt.Errorf("missing password")
	}

	nickname, ok := reqData["nickname"].(string)
	if !ok || nickname == "" {
		util.WriteBadRequest(r, "昵称不能为空")
		return fmt.Errorf("missing nickname")
	}

	// 参数长度验证
	if len(username) < 4 || len(username) > 12 {
		util.WriteBadRequest(r, "用户名长度必须在4-12个字符之间")
		return fmt.Errorf("invalid username length")
	}

	if len(password) < 6 || len(password) > 20 {
		util.WriteBadRequest(r, "密码长度必须在6-20个字符之间")
		return fmt.Errorf("invalid password length")
	}

	if len(nickname) < 2 || len(nickname) > 20 {
		util.WriteBadRequest(r, "昵称长度必须在2-20个字符之间")
		return fmt.Errorf("invalid nickname length")
	}

	return nil
}
