package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	v15 "jh_gateway/api/frontend/public/v1"
	v16 "jh_gateway/api/frontend/user/v1"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/util"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// writeProtobufResponse 统一处理protobuf响应的JSON序列化
func writeProtobufResponse(ctx context.Context, r *ghttp.Request, pbMessage proto.Message) error {
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true, // 使用proto字段名（下划线格式）保持与原API一致
		EmitUnpopulated: true, // 输出零值字段，确保空数组显示为[]而不是null
	}

	jsonData, err := marshaler.Marshal(pbMessage)
	if err != nil {
		util.LogWithTrace(ctx, "error", "protobuf JSON marshal failed: %v", err)
		util.WriteInternalError(r, "数据序列化失败")
		return err
	}

	// 将JSON字符串解析为map，然后返回
	var responseData map[string]interface{}
	if err := json.Unmarshal(jsonData, &responseData); err != nil {
		util.LogWithTrace(ctx, "error", "JSON parse to map failed: %v", err)
		// 如果解析失败，直接返回原始响应
		util.WriteSuccess(r, pbMessage)
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, responseData)
	return nil
}

// callBackendGRPCMethod 根据 HTTP 路径调用对应的 gRPC 方法
func callBackendGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文 - 只需要在这里添加一次
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return backendRoutes.Handle(ctx, conn, r)
}

// shouldValidateEarly 判断是否需要提前验证参数
func shouldValidateEarly(path, method string) bool {
	// 文件上传接口使用 multipart/form-data，不需要 JSON 验证
	if strings.HasSuffix(path, "/upload-image") && method == "POST" {
		return false
	}

	// 对于其他 admin 相关的 POST 请求，提前验证
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

	// 添加用户名
	if usernameVar := r.GetCtxVar("username"); usernameVar != nil {
		if username := usernameVar.String(); username != "" {
			md.Set("username", username)
			util.LogWithTrace(ctx, "debug", "添加 username 到 gRPC metadata: %s", username)
		}
	}

	// 添加站点ID（从域名解析中间件获取）
	if siteIdVar := r.GetCtxVar("site_id"); siteIdVar != nil {
		if id := siteIdVar.Int64(); id > 0 {
			md.Set("site_id", fmt.Sprint(id))
			util.LogWithTrace(ctx, "debug", "添加 site_id 到 gRPC metadata: %d", id)
		}
	}

	// 添加站点代码（从域名解析中间件获取）
	if siteCodeVar := r.GetCtxVar("site_code"); siteCodeVar != nil {
		if code := siteCodeVar.String(); code != "" {
			md.Set("site_code", code)
			util.LogWithTrace(ctx, "debug", "添加 site_code 到 gRPC metadata: %s", code)
		}
	}

	// 添加货币（从域名解析中间件获取）
	if currencyVar := r.GetCtxVar("currency"); currencyVar != nil {
		if currency := currencyVar.String(); currency != "" {
			md.Set("currency", currency)
			util.LogWithTrace(ctx, "debug", "添加 currency 到 gRPC metadata: %s", currency)
		}
	}

	// 添加域名（从域名解析中间件获取）
	if domainVar := r.GetCtxVar("domain"); domainVar != nil {
		if domain := domainVar.String(); domain != "" {
			md.Set("domain", domain)
			util.LogWithTrace(ctx, "debug", "添加 domain 到 gRPC metadata: %s", domain)
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

// 辅助函数：从 map 中安全获取 int64 类型的值
func getInt64FromMap(data map[string]interface{}, key string) int64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case int32:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

// callBalanceGRPCMethod 根据 HTTP 路径调用对应的 Balance gRPC 方法
func callBalanceGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling Balance gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return balanceRoutes.Handle(ctx, conn, r)
}

// callGameGRPCMethod 根据 HTTP 路径调用对应的 Game gRPC 方法
func callGameGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling Game gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return gameRoutes.Handle(ctx, conn, r)
}

// validateCreateAdRequestInHandler 在 handler 中验证创建广告请求参数
func validateCreateAdRequestInHandler(r *ghttp.Request, reqData map[string]interface{}) error {
	// 验证标题
	title, ok := reqData["title"].(string)
	if !ok || title == "" {
		util.WriteBadRequest(r, "请输入广告标题")
		return fmt.Errorf("missing title")
	}

	// 验证图片
	image, ok := reqData["image"].(string)
	if !ok || image == "" {
		util.WriteBadRequest(r, "请上传广告图片")
		return fmt.Errorf("missing image")
	}

	// 验证位置
	position, ok := reqData["position"]
	if !ok || position == "" {
		util.WriteBadRequest(r, "请选择广告位置")
		return fmt.Errorf("missing position")
	}

	// 验证开始时间
	startTime, ok := reqData["start_time"].(string)
	if !ok || startTime == "" {
		util.WriteBadRequest(r, "请选择开始时间")
		return fmt.Errorf("missing start_time")
	}

	// 验证过期时间
	expiredTime, ok := reqData["expired_time"].(string)
	if !ok || expiredTime == "" {
		util.WriteBadRequest(r, "请选择过期时间")
		return fmt.Errorf("missing expired_time")
	}

	return nil
}

// callManageGRPCMethod 根据 HTTP 路径调用对应的 Manage gRPC 方法
func callManageGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling Manage gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return manageRoutes.Handle(ctx, conn, r)
}

// AdminController 管理员控制器 - 用于直接调用jh_manage_service的方法
type AdminController struct{}

// GetMenus 获取管理员菜单列表
func (*AdminController) GetMenus(ctx context.Context, req interface{}) (string, error) {
	// 这里应该调用jh_manage_service的GetMenusJSON方法
	// 由于架构限制，暂时返回示例数据
	// 实际项目中需要通过服务发现或直接调用来获取真实数据
	return `{"code":200,"msg":"success","data":{"menus":[]}}`, nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK快速注册
 * @description 用户快速注册接口
 * @method post
 * @url /api/frontend/app/quickRegister
 * @param code 必选 string 验证码
 * @param time 必选 string 时间戳
 * @param domain 可选 string 注册域名
 * @return {"code":0,"msg":"success","data":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...","username":"a1b2c3","password":"a1b2c3"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.token string JWT token
 * @return_param data.username string 用户名
 * @return_param data.password string 密码
 * @return_param msg string 提示说明
 * @remark 快速注册会自动生成随机用户名和密码
 * @number 100
 */
func callQuickRegister(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	time, _ := reqData["time"].(string)
	domain, _ := reqData["domain"].(string)

	if time == "" {
		util.WriteBadRequest(r, "时间戳必填")
		return nil
	}

	util.LogWithTrace(ctx, "info", "calling gRPC QuickRegister with time=%s", time)

	// 创建 gRPC 客户端
	client := v15.NewPublicClient(conn)
	req := &v15.QuickRegisterReq{
		Time:   time,
		Domain: domain,
	}

	// 调用 gRPC 服务
	res, err := client.QuickRegister(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "验证码") {
			util.WriteForbidden(r, "验证码错误")
		} else if strings.Contains(err.Error(), "注册失败") {
			util.WriteInternalError(r, "注册失败，请稍后重试")
		} else {
			util.WriteInternalError(r, "注册服务暂时不可用，请稍后重试")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"token":              res.Token,
		"username":           res.Username,
		"refresh_token":      res.RefreshToken,
		"expires_in":         res.ExpiresIn,
		"refresh_expires_in": res.RefreshExpiresIn,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台
 * @title 用户注册
 * @description 用户注册接口
 * @method post
 * @url /api/frontend/app/register
 * @param username 必选 string 用户名
 * @param password 必选 string 密码
 * @param mobile 可选 string 手机号
 * @param code 必选 string 图片验证码
 * @param time 必选 string 时间戳
 * @param phone_code 可选 string 手机验证码
 * @param pay_password 可选 string 资金密码
 * @param realname 可选 string 真实姓名
 * @param email 可选 string 邮箱
 * @param birthday 可选 string 生日
 * @param agent_salt 可选 string 代理码
 * @param domain 可选 string 注册域名
 * @return {"code":0,"msg":"success","data":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 101
 */
func callUserRegister(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	// 提取字段
	username, _ := reqData["username"].(string)
	password, _ := reqData["password"].(string)
	mobile, _ := reqData["mobile"].(string)
	code, _ := reqData["code"].(string)
	time, _ := reqData["time"].(string)
	phoneCode, _ := reqData["phone_code"].(string)
	payPassword, _ := reqData["pay_password"].(string)
	realname, _ := reqData["realname"].(string)
	email, _ := reqData["email"].(string)
	birthday, _ := reqData["birthday"].(string)
	agentSalt, _ := reqData["agent_salt"].(string)
	domain, _ := reqData["domain"].(string)

	if username == "" {
		util.WriteBadRequest(r, "用户名必填")
		return nil
	}
	if password == "" {
		util.WriteBadRequest(r, "密码必填")
		return nil
	}
	if time == "" {
		util.WriteBadRequest(r, "时间戳必填")
		return nil
	}

	util.LogWithTrace(ctx, "info", "calling gRPC Register with username=%s", username)

	// 创建 gRPC 客户端
	client := v15.NewPublicClient(conn)
	req := &v15.RegisterReq{
		Username:    username,
		Password:    password,
		Mobile:      mobile,
		Code:        code,
		Time:        time,
		PhoneCode:   phoneCode,
		PayPassword: payPassword,
		Realname:    realname,
		Email:       email,
		Birthday:    birthday,
		AgentSalt:   agentSalt,
		Domain:      domain,
	}

	// 调用 gRPC 服务
	res, err := client.Register(ctx, req)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "验证码") {
			util.WriteForbidden(r, errMsg)
		} else if strings.Contains(errMsg, "已经被注册") {
			util.WriteBadRequest(r, errMsg)
		} else if strings.Contains(errMsg, "注册失败") {
			util.WriteInternalError(r, "注册失败，请稍后重试")
		} else {
			util.WriteInternalError(r, "注册服务暂时不可用，请稍后重试")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"token":              res.Token,
		"refresh_token":      res.RefreshToken,
		"expires_in":         res.ExpiresIn,
		"refresh_expires_in": res.RefreshExpiresIn,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK刷新token
 * @description 使用refresh_token刷新access_token
 * @method post
 * @url /api/frontend/app/refresh-token
 * @param refresh_token 必选 string 刷新token
 * @return {"code":0,"msg":"success","data":{"token":"xxx","refresh_token":"yyy","expires_in":3600,"refresh_expires_in":2592000}}
 * @return_param code int 状态码
 * @return_param data object 响应数据
 * @return_param msg string 提示说明
 * @remark token过期后使用refresh_token刷新，每次刷新都会延长refresh_token有效期
 * @number 102.5
 */
func callRefreshToken(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	refreshToken, _ := reqData["refresh_token"].(string)
	if refreshToken == "" {
		util.WriteBadRequest(r, "refresh_token必填")
		return nil
	}

	util.LogWithTrace(ctx, "info", "calling gRPC RefreshToken")

	client := v15.NewPublicClient(conn)
	req := &v15.RefreshTokenReq{
		RefreshToken: refreshToken,
	}

	res, err := client.RefreshToken(ctx, req)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "无效") || strings.Contains(errMsg, "过期") {
			util.WriteUnauthorized(r, "refresh token无效或已过期，请重新登录")
		} else if strings.Contains(errMsg, "用户不存在") || strings.Contains(errMsg, "已禁用") {
			util.WriteUnauthorized(r, "用户不存在或已禁用")
		} else {
			util.WriteInternalError(r, "刷新token失败")
		}
		return nil
	}

	util.WriteSuccess(r, map[string]interface{}{
		"token":              res.Token,
		"refresh_token":      res.RefreshToken,
		"expires_in":         res.ExpiresIn,
		"refresh_expires_in": res.RefreshExpiresIn,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK获取用户信息
 * @description 获取当前登录用户的详细信息
 * @method get
 * @url /api/frontend/app/user-info
 * @header Authorization 必选 string Bearer {token}
 * @return {"code":0,"msg":"success","data":{"username":"test001","realname":"张三","email":"test@example.com","mobile":"13800138000","balance":"1000.00","grade_name":"普通会员",...}}
 * @return_param code int 状态码
 * @return_param data object 用户信息
 * @return_param msg string 提示说明
 * @remark 需要登录后访问，通过JWT token获取用户信息
 * @number 103
 */
func callGetUserInfo(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetUserInfo")

	client := v16.NewUserClient(conn)
	req := &v16.GetUserInfoReq{}

	res, err := client.GetUserInfo(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "未登录") {
			util.WriteUnauthorized(r, "请先登录")
		} else if strings.Contains(err.Error(), "用户不存在") {
			util.WriteNotFound(r, "用户不存在")
		} else {
			util.WriteInternalError(r, "获取用户信息失败")
		}
		return nil
	}

	// 返回成功响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK用户退出登录
 * @description 用户退出登录接口
 * @method post
 * @url /api/frontend/app/logout
 * @header Authorization 必选 string Bearer {token}
 * @return {"code":0,"msg":"success","data":{"message":"退出成功"}}
 * @return_param code int 状态码
 * @return_param data object 响应数据
 * @return_param msg string 提示说明
 * @remark 需要登录后访问，JWT模式下主要由前端删除token完成
 * @number 104
 */
func callUserLogout(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Logout")

	client := v16.NewUserClient(conn)
	req := &v16.LogoutReq{}

	res, err := client.Logout(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "退出登录失败")
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK绑定邮箱
 * @description 绑定邮箱接口
 * @method post
 * @url /api/frontend/app/bind-email
 * @param token 必选 string token
 * @param email 必选 string 邮箱地址
 * @return {"code":0,"msg":"success","data":{"message":"绑定邮箱成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.message string 提示消息
 * @number 105
 */
func callBindEmail(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC BindEmail")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteInternalError(r, "解析请求数据失败")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.BindEmailReq{
		Email: gconv.String(reqData["email"]),
	}

	res, err := client.BindEmail(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK绑定手机
 * @description 绑定手机接口
 * @method post
 * @url /api/frontend/app/bind-mobile
 * @param token 必选 string token
 * @param mobile 必选 string 手机号
 * @param code 必选 string 手机验证码
 * @return {"code":0,"msg":"success","data":{"message":"绑定手机号成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.message string 提示消息
 * @number 106
 */
func callBindMobile(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC BindMobile")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteInternalError(r, "解析请求数据失败")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.BindMobileReq{
		Mobile: gconv.String(reqData["mobile"]),
	}

	res, err := client.BindMobile(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK修改登录密码
 * @description 修改登录密码接口
 * @method post
 * @url /api/frontend/app/change-login-password
 * @param token 必选 string token
 * @param old_password 必选 string 原密码
 * @param new_password 必选 string 新密码
 * @return {"code":0,"msg":"success","data":{"message":"修改登录密码成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.message string 提示消息
 * @number 107
 */
func callChangeLoginPassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ChangeLoginPassword")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteInternalError(r, "解析请求数据失败")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.ChangeLoginPasswordReq{
		OldPassword: gconv.String(reqData["old_password"]),
		NewPassword: gconv.String(reqData["new_password"]),
	}

	res, err := client.ChangeLoginPassword(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK绑定登录密码
 * @description 绑定登录密码接口（针对快速注册用户）
 * @method post
 * @url /api/frontend/app/bind-login-password
 * @param token 必选 string token
 * @param new_password 必选 string 新密码（6-20位）
 * @return {"code":0,"msg":"success","data":{"message":"绑定登录密码成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.message string 提示消息
 * @number 108
 */
func callBindLoginPassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC BindLoginPassword")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteInternalError(r, "解析请求数据失败")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.BindLoginPasswordReq{
		NewPassword: gconv.String(reqData["new_password"]),
	}

	res, err := client.BindLoginPassword(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"message": res.Message,
	})
	return nil
}

// callFrontendGRPCMethod 根据 HTTP 路径调用对应的前台 gRPC 方法
func callFrontendGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文
	ctx = addTraceToContext(ctx, r)

	// 前台接口需要添加域名解析信息到gRPC metadata
	// 域名解析中间件已经将site_id和site_code添加到context中
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling frontend gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return frontendRoutes.Handle(ctx, conn, r)
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK用户登录
 * @description 用户登录接口
 * @method post
 * @url /api/frontend/app/login
 * @param username 必选 string 用户名
 * @param password 必选 string 密码
 * @param code 可选 string 验证码
 * @param time 可选 string 时间戳
 * @return {"code":0,"msg":"success","data":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.token string JWT token
 * @return_param msg string 提示说明
 * @remark 用户登录成功后返回JWT token
 * @number 101
 */
func callUserLogin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	username, _ := reqData["username"].(string)
	password, _ := reqData["password"].(string)
	code, _ := reqData["code"].(string)
	time, _ := reqData["time"].(string)

	if username == "" || password == "" {
		util.WriteBadRequest(r, "用户名和密码必填")
		return nil
	}

	util.LogWithTrace(ctx, "info", "calling gRPC Login with username=%s", username)

	client := v15.NewPublicClient(conn)
	req := &v15.LoginReq{
		Username: username,
		Password: password,
		Code:     code,
		Time:     time,
	}

	res, err := client.Login(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "用户名或密码错误") {
			util.WriteForbidden(r, "用户名或密码错误")
		} else if strings.Contains(err.Error(), "验证码") {
			util.WriteForbidden(r, "验证码错误")
		} else {
			util.WriteInternalError(r, "登录服务暂时不可用，请稍后重试")
		}
		return nil
	}

	util.WriteSuccess(r, map[string]interface{}{
		"token":              res.Token,
		"refresh_token":      res.RefreshToken,
		"expires_in":         res.ExpiresIn,
		"refresh_expires_in": res.RefreshExpiresIn,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK获取验证码
 * @description 生成验证码图片
 * @method get
 * @url /api/frontend/app/captcha?time=1234567890
 * @param time 必选 string 时间戳
 * @return 验证码图片（image/jpeg）
 * @remark 返回验证码图片，验证码有效期5分钟
 * @number 102
 */
func callGenerateCaptcha(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	time := r.Get("time").String()

	if time == "" {
		util.WriteBadRequest(r, "时间戳必填")
		return nil
	}

	util.LogWithTrace(ctx, "info", "calling gRPC GenerateCaptcha with time=%s", time)

	client := v15.NewPublicClient(conn)
	req := &v15.GenerateCaptchaReq{
		Time: time,
	}

	res, err := client.GenerateCaptcha(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "GenerateCaptcha gRPC error: %v", err)
		util.WriteInternalError(r, "生成验证码失败")
		return nil
	}

	util.LogWithTrace(ctx, "info", "GenerateCaptcha success, image size: %d bytes, content_type: %s", len(res.Image), res.ContentType)

	// 检查图片数据是否为空
	if len(res.Image) == 0 {
		util.LogWithTrace(ctx, "error", "验证码图片数据为空")
		util.WriteInternalError(r, "验证码图片生成失败")
		return nil
	}

	// 清除之前可能设置的响应头
	r.Response.ClearBuffer()

	// 设置响应头
	r.Response.Header().Set("Content-Type", res.ContentType)
	r.Response.Header().Set("Cache-Control", "no-cache, must-revalidate")
	r.Response.Header().Set("Content-Length", fmt.Sprintf("%d", len(res.Image)))

	util.LogWithTrace(ctx, "info", "准备写入图片数据: %d bytes", len(res.Image))

	// 直接写入图片数据并退出
	r.Response.Write(res.Image)
	r.ExitAll()

	util.LogWithTrace(ctx, "info", "图片数据已写入响应")
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK获取会员等级列表
 * @description 获取会员等级列表接口
 * @method get
 * @url /api/frontend/app/grade-list
 * @param token 必选 string token
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"name":"普通会员","points_upgrade":0,"bonus_upgrade":"0.00","bonus_birthday":"0.00","rebate_percent_sports":"0.00","rebate_percent_lottery":"0.00","rebate_percent_live":"0.00","rebate_percent_egame":"0.00","rebate_percent_poker":"0.00"}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.list array 等级列表
 * @return_param data.list.id int 等级ID
 * @return_param data.list.name string 等级名称
 * @return_param data.list.points_upgrade int 升级所需积分
 * @return_param data.list.bonus_upgrade string 升级赠送彩金
 * @return_param data.list.bonus_birthday string 生日彩金
 * @return_param data.list.rebate_percent_sports string 体育返水比例
 * @return_param data.list.rebate_percent_lottery string 彩票返水比例
 * @return_param data.list.rebate_percent_live string 真人视讯返水比例
 * @return_param data.list.rebate_percent_egame string 电子游戏返水比例
 * @return_param data.list.rebate_percent_poker string 棋牌返水比例
 * @number 110
 */
func callGetAppGradeList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetGradeList")

	client := v16.NewUserClient(conn)
	req := &v16.GetGradeListReq{}

	res, err := client.GetGradeList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/网站
 * @title OK获取站点设置
 * @description 获取站点设置接口
 * @method get
 * @url /api/frontend/app/setting-site
 * @return {"code":0,"msg":"success","data":{"agent_url":"","site_name":"测试站点","mobile_url":"","switch_register":1,"switch_grade":1,"service_url":"","close":0,"min_withdraw":"100.00","max_withdraw":"50000.00","mobile_logo":"","close_reason":"您好，系统正在维护中，估计需要1个小时左右，给您造成不便，敬请谅解！"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.agent_url string 代理url
 * @return_param data.site_name string 站点名称
 * @return_param data.mobile_url string 手机url
 * @return_param data.switch_register int 会员注册开关 0=关闭 1=开启
 * @return_param data.switch_grade int 会员等级开关 0=关闭 1=开启
 * @return_param data.service_url string 客服链接
 * @return_param data.close int 站点关闭开关 0=正常 1=关闭
 * @return_param data.min_withdraw string 单笔最小提现金额
 * @return_param data.max_withdraw string 单笔最大提现金额
 * @return_param data.mobile_logo string 手机端logo
 * @return_param data.close_reason string 站点关闭理由
 * @number 111
 */
func callGetSiteSetting(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetSiteSetting")

	client := v15.NewPublicClient(conn)
	req := &v15.GetSiteSettingReq{}

	res, err := client.GetSiteSetting(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/运营
 * @title OK获取公告列表
 * @description 获取最新公告列表接口
 * @method get
 * @url /api/frontend/app/notice-list
 * @param limit 可选 int 限制数量，默认3
 * @return {"code":0,"msg":"success","data":{"list":[{"title":"公告标题","url":"","content":"公告内容","created_at":"2025-01-19 10:00:00"}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.list array 公告列表
 * @return_param data.list.title string 标题
 * @return_param data.list.url string 链接
 * @return_param data.list.content string 内容
 * @return_param data.list.created_at string 创建时间
 * @number 111
 */
func callGetAppNoticeList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetAppNoticeList")

	client := v15.NewPublicClient(conn)
	req := &v15.GetNoticeListReq{
		Limit: r.Get("limit", 3).Int32(),
	}

	res, err := client.GetAppNoticeList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/运营
 * @title OK获取广告列表
 * @description 获取首页广告列表接口
 * @method get
 * @url /api/frontend/app/ad-list
 * @param mobile 可选 int 是否手机端 0=PC 1=手机，默认0
 * @return {"code":0,"msg":"success","data":{"list":[{"name":"广告名称","image":"图片地址","url":"链接","type":1,"before_login":1,"position":1,"sort":0}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.list array 广告列表
 * @return_param data.list.name string 名称
 * @return_param data.list.image string 图片地址
 * @return_param data.list.url string 链接
 * @return_param data.list.type int 类型 1=首页图片弹窗 2=首页悬浮图片 3=首页banner图片
 * @return_param data.list.before_login int 登录前显示 0=登录后 1=登录前
 * @return_param data.list.position int 位置 1=PC 2=WAP
 * @return_param data.list.sort int 排序
 * @number 112
 */
func callGetAppAdList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetAppAdList")

	client := v15.NewPublicClient(conn)
	req := &v15.GetAdListReq{
		Mobile: r.Get("mobile", 0).Int32(),
	}

	res, err := client.GetAppAdList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}
