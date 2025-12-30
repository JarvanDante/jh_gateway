package proxy

import (
	"context"
	"fmt"
	"jh_gateway/api/backend/admin/v1"
	v2 "jh_gateway/api/backend/role/v1"
	v3 "jh_gateway/api/backend/site/v1"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/tracing"
	"jh_gateway/internal/util"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// AdminGRPCToHTTP 将 HTTP 请求转换为 gRPC 调用
func AdminGRPCToHTTP(serviceName string) ghttp.HandlerFunc {
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
		if err := callAdminGRPCMethod(ctx, conn, r); err != nil {
			tracing.SetSpanError(span, err)
			util.WriteInternalError(r, err.Error())
			return
		}

		tracing.SetSpanAttributes(span, attribute.Bool("success", true))
	}
}

// callAdminGRPCMethod 根据 HTTP 路径调用对应的 gRPC 方法
func callAdminGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	path := r.URL.Path
	method := r.Method

	// 统一添加traceId到gRPC上下文 - 只需要在这里添加一次
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling gRPC method for path: %s, method: %s", path, method)

	// 管理员相关接口
	switch {
	//后台登录
	case strings.HasSuffix(path, "/login") && method == "POST":
		return callAdminLogin(ctx, conn, r)
		//刷新token
	case strings.HasSuffix(path, "/refresh-token") && method == "GET":
		return callAdminRefreshToken(ctx, conn, r)
		//获取员工列表
	case strings.HasSuffix(path, "/admins") && method == "GET":
		return callGetAdminList(ctx, conn, r)
		//添加员工
	case strings.HasSuffix(path, "/create-admin") && method == "POST":
		return callAdminCreate(ctx, conn, r)
		//编辑员工
	case strings.HasSuffix(path, "/update-admin") && method == "POST":
		return callUpdateAdmin(ctx, conn, r)
		//删除员工
	case strings.HasSuffix(path, "/delete-admin") && method == "POST":
		return callDeleteAdmin(ctx, conn, r)
		//退出登录
	case strings.HasSuffix(path, "/logout") && method == "POST":
		return callAdminLogout(ctx, conn, r)
		//修改密码
	case strings.HasSuffix(path, "/change-password") && method == "POST":
		return callAdminChangePassword(ctx, conn, r)
		//站点配置信息
	case strings.HasSuffix(path, "/basic-setting") && method == "GET":
		return callGetBasicSetting(ctx, conn, r)
		//更新站点配置信息
	case strings.HasSuffix(path, "/update-basic-setting") && method == "POST":
		return callUpdateBasicSetting(ctx, conn, r)
		//职务列表
	case strings.HasSuffix(path, "/roles") && method == "GET":
		return callGetRoleList(ctx, conn, r)
		//添加职务
	case strings.HasSuffix(path, "/create-role") && method == "POST":
		return callCreateRole(ctx, conn, r)
		//编辑职务
	case strings.HasSuffix(path, "/update-role") && method == "POST":
		return callUpdateRole(ctx, conn, r)
		//删除职务
	case strings.HasSuffix(path, "/delete-role") && method == "POST":
		return callDeleteRole(ctx, conn, r)
	}

	// 用户相关接口已删除
	return fmt.Errorf("unsupported path: %s", path)
}

/**
 * showdoc
 * @catalog 后台
 * @title 登录
 * @description 员工登录的接口
 * @method post
 * @url /api/admin/login
 * @param username 必选 string 用户名
 * @param password 必选 string 密码
 * @param code 可选 string 动态验证码
 * @return {"code":0,"msg":"success","data":{"socket":"","token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhZG1pbl9pZCI6MjcsImV4cCI6MTc2NzA4NDI5MSwiaWF0IjoxNzY2OTk3ODkxLCJzaXRlX2lkIjoxLCJ1c2VybmFtZSI6Im1pY2hhZWwifQ.FvPBCn9I_vyD5zgNBxIpb2xPDNbYQ3NWpJj2EL99g6I"}}
 * @return_param code int 状态码
 * @return_param data string 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callAdminLogin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
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
	client := v1.NewAdminClient(conn)
	req := &v1.LoginReq{
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

/**
 * showdoc
 * @catalog 后台
 * @title 刷新Token
 * @description 刷新管理员登录token的接口
 * @method get
 * @url /api/admin/refresh-token
 * @param Authorization 必选 string Bearer token (Header中)
 * @return {"code":0,"msg":"success","data":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhZG1pbl9pZCI6MjcsImV4cCI6MTc2NzIwNjg5NCwiaWF0IjoxNzY3MTIwNDk0LCJzaXRlX2lkIjoxLCJ1c2VyX2lkIjowLCJ1c2VybmFtZSI6Im1pY2hhZWwifQ.KCuhEKfidhecigJP-W5O9F8LaOligchsVy6QgugqTwA"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.token string 新的JWT token
 * @return_param msg string 提示说明
 * @remark 需要在Header中提供有效的JWT token，返回新的token用于延长登录状态
 * @number 2
 */
// callAdminRefreshToken 处理管理员token刷新
func callAdminRefreshToken(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin RefreshToken")

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.RefreshTokenReq{}

	// 调用 gRPC 服务
	res, err := client.RefreshToken(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "未登录或登录已过期") ||
			strings.Contains(err.Error(), "管理员不存在") ||
			strings.Contains(err.Error(), "账号已被禁用") {
			util.WriteUnauthorized(r, "登录已过期，请重新登录")
		} else {
			util.WriteInternalError(r, "刷新token失败，请重新登录")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"token": res.Token,
	})
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 添加员工
 * @description 添加员工
 * @method post
 * @url /api/admin/create-admin
 * @param Authorization 必选 string Bearer token (Header中)
 * @return {"code":0,"msg":"success","data":{"message":"管理员创建成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.message string message
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 2
 */
func callAdminCreate(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
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
	client := v1.NewAdminClient(conn)
	req := &v1.CreateAdminReq{
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
		(strings.HasSuffix(path, "/create-admin") && method == "POST") ||
		(strings.HasSuffix(path, "/update-admin") && method == "POST") ||
		(strings.HasSuffix(path, "/delete-admin") && method == "POST") ||
		(strings.HasSuffix(path, "/update-basic-setting") && method == "POST")
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

// addUserInfoToGRPCContext 将用户信息添加到 gRPC metadata 中
func addUserInfoToGRPCContext(ctx context.Context, r *ghttp.Request) context.Context {
	md := metadata.New(map[string]string{})

	// 添加管理员ID
	if adminIdVar := r.GetCtxVar("admin_id"); adminIdVar != nil {
		if id := adminIdVar.Int(); id > 0 {
			md.Set("admin_id", fmt.Sprint(id))
			util.LogWithTrace(ctx, "debug", "添加 admin_id 到 gRPC metadata: %d", id)
		}
	}

	// 添加用户ID
	if userIdVar := r.GetCtxVar("user_id"); userIdVar != nil {
		if id := userIdVar.Int(); id > 0 {
			md.Set("user_id", fmt.Sprint(id))
			util.LogWithTrace(ctx, "debug", "添加 user_id 到 gRPC metadata: %d", id)
		}
	}

	// 添加客户端IP
	if clientIP := r.GetClientIp(); clientIP != "" {
		md.Set("client_ip", clientIP)
		util.LogWithTrace(ctx, "debug", "添加 client_ip 到 gRPC metadata: %s", clientIP)
	}

	// 如果有metadata，则创建新的outgoing context
	if len(md) > 0 {
		return metadata.NewOutgoingContext(ctx, md)
	}

	return ctx
}

// validateAndParseRequest 验证并解析请求参数
// 注意：这个函数现在主要用于向后兼容，推荐使用 RequestParser 中间件
func validateAndParseRequest(ctx context.Context, r *ghttp.Request) (map[string]interface{}, error) {
	// 使用中间件的辅助函数获取请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
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

/**
 * showdoc
 * @catalog 后台/系统/全局设置
 * @title 获取站点配置信息
 * @description 获取站点配置信息
 * @method get
 * @url /api/admin/basic-setting
 * @param token 必选 string 员工token
 * @return {"code":0,"msg":"success","data":{"code":"site_1","name":"站点_1","register_time_interval":1179844069,"switch_register":true,"close_reason":"aute Duis dolor dolor","service_url":"http://fruy.中国互联.公司/yfxohunw","agent_url":"http://bngmrpdvfm.ee/ixn","mobile_url":"http://efoccuis.aq/nikrlrw","agent_register_url":"http://vuwku.li/rcxpcsphf","min_withdraw":79,"max_withdraw":86,"mobile_logo":"18110356852"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data array 数组
 * @return_param data.code string 应用代码
 * @return_param data.name string 应用名称
 * @return_param data.register_time_interval int 同一IP重复注册次数
 * @return_param data.switch_register int 是否开放注册
 * @return_param data.is_close int 是否关站
 * @return_param data.close_reason string 关站提示语
 * @return_param data.service_url string 客服链接
 * @return_param data.agent_url string 代理链接
 * @return_param data.mobile_url string APP下载地址
 * @return_param data.agent_register_url string 代理推广地址
 * @return_param data.min_withdraw string 默认单笔最小提现金额
 * @return_param data.max_withdraw string 默认单笔最大提现金额
 * @return_param data.game_free_play string 游戏试玩地址
 * @return_param data.mobile_logo string 手机logo地址
 * @return_param data.default_agent_id string 默认代理ID
 * @return_param data.default_agent_name string 默认代理名称
 * @return_param data.balance string 默认额度
 * @return_param data.balance_reset string 可用额度
 * @remark 备注
 * @number 1
 */
func callGetBasicSetting(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Site GetBasicSetting")

	// 创建 gRPC 客户端
	client := v3.NewSiteClient(conn)
	req := &v3.GetBasicSettingReq{
		SiteId: r.Get("site_id", 1).Int32(),
	}

	// 调用 gRPC 服务
	res, err := client.GetBasicSetting(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取基本设置失败，请稍后重试")
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/全局设置
 * @title 设置站点配置信息
 * @description 设置站点配置信息
 * @method post
 * @url /api/admin/update-basic-setting
 * @param token 必选 string 员工token
 * @param site_id 必选 int 站点id
 * @param register_time_interval 必选 int 同一IP重复注册时间间隔
 * @param switch_register 必选 bool 同一IP重复注册时间间隔
 * @param is_close 必选 bool 是否关闭站点
 * @param close_reason 必选 string 关闭原因
 * @param url_agent_pc 必选 string 代理链接地址
 * @param url_mobile 必选 string 手机域名地址
 * @param url_agent_register 必选 string 代理推广地址
 * @param min_withdraw 必选 int 单笔最低提现金额
 * @param max_withdraw 必选 int 单笔最高提现金额
 * @param mobile_logo 必选 string 手机端Logo
 * @param url_service 必选 string 客服链接
 * @return {"code":0,"msg":"success","data":{"message":"设置成功"}}
 * @return_param code int 状态码
 * @return_param message string 提示说明
 * @return_param data array 数组
 * @remark 备注
 * @number 1
 */
func callUpdateBasicSetting(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	util.LogWithTrace(ctx, "info", "calling gRPC Site UpdateBasicSetting")

	// 创建 gRPC 客户端
	client := v3.NewSiteClient(conn)
	req := &v3.UpdateBasicSettingReq{
		SiteId:               getInt32FromMap(reqData, "site_id"),
		RegisterTimeInterval: getInt32FromMap(reqData, "register_time_interval"),
		SwitchRegister:       getBoolFromMap(reqData, "switch_register"),
		IsClose:              getBoolFromMap(reqData, "is_close"),
		CloseReason:          getStringFromMap(reqData, "close_reason"),
		UrlAgentPc:           getStringFromMap(reqData, "url_agent_pc"),
		UrlMobile:            getStringFromMap(reqData, "url_mobile"),
		UrlAgentRegister:     getStringFromMap(reqData, "url_agent_register"),
		MinWithdraw:          getInt32FromMap(reqData, "min_withdraw"),
		MaxWithdraw:          getInt32FromMap(reqData, "max_withdraw"),
		MobileLogo:           getStringFromMap(reqData, "mobile_logo"),
		UrlService:           getStringFromMap(reqData, "url_service"),
	}

	// 如果没有指定站点ID，使用默认值
	if req.SiteId == 0 {
		req.SiteId = 1
	}

	// 调用 gRPC 服务
	res, err := client.UpdateBasicSetting(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "更新基本设置失败，请稍后重试")
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": res.Message,
	})
	return nil
}

// 辅助函数：从 map 中安全获取各种类型的值
func getStringFromMap(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt32FromMap(data map[string]interface{}, key string) int32 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int32:
			return v
		case int:
			return int32(v)
		case float64:
			return int32(v)
		}
	}
	return 0
}

func getBoolFromMap(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func getFloat64FromMap(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int32:
			return float64(v)
		}
	}
	return 0.0
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 获取角色列表
 * @description 获取站点角色列表
 * @method get
 * @url /api/admin/roles
 * @param token 必选 string 员工token
 * @return {"code":0,"msg":"success","data":{"roles":[{"id":1,"site_id":1,"name":"管理员","status":1,"created_at":1640995200,"updated_at":1640995200,"permissions":"1,2,3"}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.roles array 角色列表
 * @return_param data.roles.id int 角色ID
 * @return_param data.roles.site_id int 站点ID
 * @return_param data.roles.name string 角色名称
 * @return_param data.roles.status int 状态：0=禁用，1=启用
 * @return_param data.roles.created_at int 创建时间戳
 * @return_param data.roles.updated_at int 更新时间戳
 * @return_param data.roles.permissions string 权限配置
 * @remark 备注
 * @number 1
 */
func callGetRoleList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Role GetRoleList")

	// 创建 gRPC 客户端
	client := v2.NewRoleClient(conn)
	req := &v2.GetRoleListReq{
		SiteId: r.Get("site_id", 1).Int32(),
	}

	// 调用 gRPC 服务
	res, err := client.GetRoleList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取角色列表失败，请稍后重试")
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 创建角色
 * @description 创建角色
 * @method post
 * @url /api/admin/create-role
 * @param site_id 必选 int 应用ID
 * @param name 必选 string 角色名称
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"创建成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callCreateRole(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Role CreateRole")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	name, ok := reqData["name"].(string)
	if !ok || name == "" {
		util.WriteBadRequest(r, "角色名称不能为空")
		return nil
	}

	siteId := int32(1) // 默认站点ID
	if s, ok := reqData["site_id"].(float64); ok {
		siteId = int32(s)
	}

	// 创建 gRPC 客户端
	client := v2.NewRoleClient(conn)
	req := &v2.CreateRoleReq{
		SiteId: siteId,
		Name:   name,
	}

	// 调用 gRPC 服务
	res, err := client.CreateRole(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "创建角色失败，请稍后重试")
		return nil
	}

	// 根据业务逻辑返回响应
	if !res.Success {
		util.WriteBadRequest(r, res.Message)
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 更新角色
 * @description 更新角色
 * @method post
 * @url /api/admin/update-role
 * @param id 必选 int 角色ID
 * @param name 必选 string 角色名称
 * @return {"code":0,"msg":"更新成功","data":{"success":true,"message":"更新成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callUpdateRole(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Role UpdateRole")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	name, ok := reqData["name"].(string)
	if !ok || name == "" {
		util.WriteBadRequest(r, "角色名称不能为空")
		return nil
	}

	id := int32(0)
	if i, ok := reqData["id"].(float64); ok {
		id = int32(i)
	}
	if id <= 0 {
		util.WriteBadRequest(r, "角色ID无效")
		return nil
	}

	siteId := int32(1) // 默认站点ID
	if s, ok := reqData["site_id"].(float64); ok {
		siteId = int32(s)
	}

	// 创建 gRPC 客户端
	client := v2.NewRoleClient(conn)
	req := &v2.UpdateRoleReq{
		Id:     id,
		SiteId: siteId,
		Name:   name,
	}

	// 调用 gRPC 服务
	res, err := client.UpdateRole(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "更新角色失败，请稍后重试")
		return nil
	}

	// 根据业务逻辑返回响应
	if !res.Success {
		util.WriteBadRequest(r, res.Message)
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 删除角色
 * @description 删除角色的接口
 * @method post
 * @url /api/admin/delete-role
 * @param id 必选 int 角色ID
 * @return {"code":0,"msg":"删除成功","data":{"success":true,"message":"删除成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callDeleteRole(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Role DeleteRole")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	id := int32(0)
	if i, ok := reqData["id"].(float64); ok {
		id = int32(i)
	}
	if id <= 0 {
		util.WriteBadRequest(r, "角色ID无效")
		return nil
	}

	siteId := int32(1) // 默认站点ID
	if s, ok := reqData["site_id"].(float64); ok {
		siteId = int32(s)
	}

	// 创建 gRPC 客户端
	client := v2.NewRoleClient(conn)
	req := &v2.DeleteRoleReq{
		Id:     id,
		SiteId: siteId,
	}

	// 调用 gRPC 服务
	res, err := client.DeleteRole(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "删除角色失败，请稍后重试")
		return nil
	}

	// 根据业务逻辑返回响应
	if !res.Success {
		util.WriteBadRequest(r, res.Message)
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台
 * @title 退出登录
 * @description 管理员退出登录的接口
 * @method post
 * @url /api/admin/logout
 * @return {"code":0,"msg":"退出成功","data":{"success":true,"message":"退出成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callAdminLogout(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin Logout")

	// logout 接口不需要请求体，直接创建空的请求
	client := v1.NewAdminClient(conn)
	req := &v1.LogoutReq{}

	// 调用 gRPC 服务
	res, err := client.Logout(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "退出登录失败，请稍后重试")
		return nil
	}

	// 根据业务逻辑返回响应
	if !res.Success {
		util.WriteBadRequest(r, res.Message)
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 管理员修改密码
 * @description 管理员修改密码
 * @method post
 * @url /api/admin/change-password
 * @param old_password 必选 string 旧密码
 * @param new_password 必选 string 新密码
 * @return {"code":0,"msg":"修改密码成功","data":{"success":true,"message":"修改密码成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callAdminChangePassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin ChangePassword")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	oldPassword, ok := reqData["old_password"].(string)
	if !ok || oldPassword == "" {
		util.WriteBadRequest(r, "请输入旧密码")
		return nil
	}

	newPassword, ok := reqData["new_password"].(string)
	if !ok || newPassword == "" {
		util.WriteBadRequest(r, "请输入新密码")
		return nil
	}

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.ChangePasswordReq{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}

	// 调用 gRPC 服务
	res, err := client.ChangePassword(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "修改密码失败，请稍后重试")
		return nil
	}

	// 根据业务逻辑返回响应
	if !res.Success {
		util.WriteBadRequest(r, res.Message)
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 获取员工列表
 * @description 获取员工列表
 * @method get
 * @url /api/admin/admins
 * @param username 可选 string 用户名筛选
 * @param status 可选 int 状态筛选
 * @param page 可选 int 页码
 * @param size 可选 int 每页数量
 * @return {"code":0,"msg":"success","data":{"list":[{"id":58,"username":"karson22139","nickname":"马军","role":1,"status":1,"created_at":"2006-01-02 15:04:05"},{"id":57,"username":"karson66","nickname":"叶强","role":1,"status":1,"last_login_ip":"127.0.0.1","last_login_time":"2006-01-02 15:04:05","created_at":"2006-01-02 15:04:05"},{"id":56,"username":"刘涛","nickname":"石娟","role":1,"status":1,"created_at":"2006-01-02 15:04:05"},{"id":55,"username":"karson","nickname":"马军","role":1,"status":1,"created_at":"2006-01-02 15:04:05"},{"id":54,"username":"xuping22","nickname":"马军","role":1,"status":1,"created_at":"2006-01-02 15:04:05"},{"id":53,"username":"xuping7","nickname":"马军","role":1,"status":1,"created_at":"2006-01-02 15:04:05"},{"id":52,"username":"xuping3","nickname":"马军","role":1,"status":1,"created_at":"2006-01-02 15:04:05"},{"id":51,"username":"xuping2","nickname":"马军","role":1,"status":1,"created_at":"2006-01-02 15:04:05"},{"id":50,"username":"xuping6","nickname":"马军","role":1,"status":1,"last_login_ip":"127.0.0.1","last_login_time":"2006-01-02 15:04:05","created_at":"2006-01-02 15:04:05"},{"id":49,"username":"xuping","nickname":"马军","role":1,"status":1,"last_login_ip":"127.0.0.1","last_login_time":"2006-01-02 15:04:05","created_at":"2006-01-02 15:04:05"}],"total":58,"page":1,"size":10}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callGetAdminList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin GetAdminList")

	// 获取查询参数
	username := r.Get("username", "").String()
	status := r.Get("status", 0).Int32()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 10).Int32()

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.GetAdminListReq{
		Username: username,
		Status:   status,
		Page:     page,
		Size:     size,
	}

	// 调用 gRPC 服务
	res, err := client.GetAdminList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取员工列表失败，请稍后重试")
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, res)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 编辑员工
 * @description 编辑员工信息
 * @method post
 * @url /api/admin/update-admin
 * @param id 必选 int 员工ID
 * @param password 可选 string 密码
 * @param nickname 可选 string 昵称
 * @param role 可选 int 角色
 * @param status 可选 int 状态
 * @return {"code":0,"msg":"编辑员工成功","data":{}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callUpdateAdmin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin UpdateAdmin")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	id := int32(0)
	if i, ok := reqData["id"].(float64); ok {
		id = int32(i)
	}
	if id <= 0 {
		util.WriteBadRequest(r, "员工ID无效")
		return nil
	}

	password, _ := reqData["password"].(string)
	nickname, _ := reqData["nickname"].(string)

	role := int32(0)
	if r, ok := reqData["role"].(float64); ok {
		role = int32(r)
	}

	status := int32(-1) // 使用-1表示不更新状态
	if s, ok := reqData["status"].(float64); ok {
		status = int32(s)
	}

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.UpdateAdminReq{
		Id:       id,
		Password: password,
		Nickname: nickname,
		Role:     role,
		Status:   status,
	}

	// 调用 gRPC 服务
	_, err = client.UpdateAdmin(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "管理员不存在") {
			util.WriteBadRequest(r, "管理员不存在")
		} else if strings.Contains(err.Error(), "数据库") {
			util.WriteInternalError(r, "数据库操作失败，请稍后重试")
		} else {
			util.WriteInternalError(r, "编辑员工失败，请稍后重试")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": "编辑员工成功",
	})
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 删除员工
 * @description 删除员工
 * @method post
 * @url /api/admin/delete-admin
 * @param id 必选 int 员工ID
 * @return {"code":0,"msg":"删除员工成功","data":{}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callDeleteAdmin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin DeleteAdmin")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	id := int32(0)
	if i, ok := reqData["id"].(float64); ok {
		id = int32(i)
	}
	if id <= 0 {
		util.WriteBadRequest(r, "员工ID无效")
		return nil
	}

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.DeleteAdminReq{
		Id: id,
	}

	// 调用 gRPC 服务
	_, err = client.DeleteAdmin(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "管理员不存在") {
			util.WriteBadRequest(r, "管理员不存在")
		} else if strings.Contains(err.Error(), "不能删除自己") {
			util.WriteBadRequest(r, "不能删除自己")
		} else if strings.Contains(err.Error(), "数据库") {
			util.WriteInternalError(r, "数据库操作失败，请稍后重试")
		} else {
			util.WriteInternalError(r, "删除员工失败，请稍后重试")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": "删除员工成功",
	})
	return nil
}
