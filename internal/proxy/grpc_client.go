package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/errors/gerror"
	"io"
	v9 "jh_gateway/api/backend/ad/v1"
	"jh_gateway/api/backend/admin/v1"
	v6 "jh_gateway/api/backend/balance/v1"
	v7 "jh_gateway/api/backend/game/v1"
	v8 "jh_gateway/api/backend/message/v1"
	v10 "jh_gateway/api/backend/notice/v1"
	v11 "jh_gateway/api/backend/option/v1"
	v2 "jh_gateway/api/backend/role/v1"
	v3 "jh_gateway/api/backend/site/v1"
	v4 "jh_gateway/api/backend/upload/v1"
	v5 "jh_gateway/api/backend/user/v1"
	v13 "jh_gateway/api/manage/admin/v1"
	v12 "jh_gateway/api/manage/customer/v1"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/util"
	"path/filepath"
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

// callAppGRPCMethod 根据 HTTP 路径调用对应的 gRPC 方法
func callAppGRPCMethod(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	// 统一添加traceId到gRPC上下文 - 只需要在这里添加一次
	ctx = addTraceToContext(ctx, r)

	// 添加用户信息到 gRPC metadata
	ctx = addUserInfoToGRPCContext(ctx, r)

	util.LogWithTrace(ctx, "info", "calling gRPC method for path: %s, method: %s", r.URL.Path, r.Method)

	// 使用路由注册表处理请求
	return appRoutes.Handle(ctx, conn, r)
}

/**
 * showdoc
 * @catalog 后台
 * @title 登录
 * @description 员工登录的接口
 * @method post
 * @url /api/backend/app/login
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
 * @url /api/backend/app/refresh-token
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
 * @catalog 后台/公共
 * @title 获取管理员信息
 * @description 获取当前登录管理员的基本信息、角色和权限菜单
 * @method get
 * @url /api/backend/app/get-info
 * @param Authorization 必选 string Bearer token (Header中)
 * @return {"code":0,"msg":"success","data":{"roles":["管理员"],"name":"michael","avatar":"https://wpimg.wallstcn.com/577965b9-bb9e-4e02-9f0c-095b41417191","introduction":"管理员 michael","menus":[{"id":1,"name":"系统管理","path":"system","type":1,"sort":0,"children":[{"id":2,"name":"管理员管理","path":"system/admin","type":1,"sort":0,"children":[{"id":3,"name":"管理员列表","path":"system/admin/list","type":1,"sort":0}]}]}]}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.roles array 角色列表
 * @return_param data.name string 管理员名称
 * @return_param data.avatar string 头像URL
 * @return_param data.introduction string 介绍
 * @return_param data.menus array 菜单权限列表
 * @return_param msg string 提示说明
 * @remark 需要在Header中提供有效的JWT token
 * @number 3
 */
func callGetInfo(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin GetInfo")

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.GetInfoReq{}

	// 调用 gRPC 服务
	res, err := client.GetInfo(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "未登录或登录已过期") ||
			strings.Contains(err.Error(), "管理员不存在") ||
			strings.Contains(err.Error(), "账号已被禁用") {
			util.WriteUnauthorized(r, "登录已过期，请重新登录")
		} else {
			util.WriteInternalError(r, "获取管理员信息失败，请稍后重试")
		}
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/公共
 * @title 获取菜单列表
 * @description 获取当前登录用户的菜单列表
 * @method get
 * @url /api/backend/app/menus
 * @param Authorization 必选 string Bearer token
 * @return {"code":0,"msg":"success","data":{"menus":[{"id":1,"name":"系统","path":"","type":1,"sort":0,"children":[{"id":2,"name":"全局设置","path":"basic-setting","type":1,"sort":0,"children":[{"id":3,"name":"基本信息","path":"user-basic-info","type":1,"sort":0}]}]}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.menus array 菜单列表
 * @remark 需要在Header中提供有效的JWT token
 * @number 4
 */
func callMenus(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin Menus")

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.MenusReq{}

	// 调用 gRPC 服务
	res, err := client.Menus(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "未登录或登录已过期") ||
			strings.Contains(err.Error(), "管理员不存在") ||
			strings.Contains(err.Error(), "账号已被禁用") {
			util.WriteUnauthorized(r, "登录已过期，请重新登录")
		} else {
			util.WriteInternalError(r, "获取菜单列表失败，请稍后重试")
		}
		return nil
	}

	// 直接返回菜单数组，而不是包装在menus字段中
	util.WriteSuccess(r, res.Menus)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 添加员工
 * @description 添加员工
 * @method post
 * @url /api/backend/app/create-admin
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

	if strings.HasSuffix(path, "/create-ad") && method == "POST" {
		if err := validateCreateAdRequestInHandler(r, reqData); err != nil {
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
 * @url /api/backend/app/basic-setting
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/全局设置
 * @title 设置站点配置信息
 * @description 设置站点配置信息
 * @method post
 * @url /api/backend/app/update-basic-setting
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
		case string:
			// 处理字符串类型的数字（来自 GET 查询参数或 form-data）
			return gconv.Int32(v)
		}
	}
	return 0
}

func getInt32FromMapWithDefault(data map[string]interface{}, key string, defaultValue int32) int32 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int32:
			return v
		case int:
			return int32(v)
		case float64:
			return int32(v)
		case string:
			// 处理字符串类型的数字（来自 GET 查询参数或 form-data）
			return gconv.Int32(v)
		}
	}
	return defaultValue
}

func getBoolFromMap(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			// 处理字符串类型的布尔值（来自 GET 查询参数或 form-data）
			return gconv.Bool(v)
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
		case string:
			// 处理字符串类型的数字（来自 GET 查询参数或 form-data）
			return gconv.Float64(v)
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
 * @url /api/backend/app/roles
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 创建角色
 * @description 创建角色
 * @method post
 * @url /api/backend/app/create-role
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 更新角色
 * @description 更新角色
 * @method post
 * @url /api/backend/app/update-role
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 删除角色
 * @description 删除角色的接口
 * @method post
 * @url /api/backend/app/delete-role
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 获取角色权限列表
 * @description 获取角色权限列表
 * @method get
 * @url /api/backend/app/permissions
 * @param token 必选 string 员工token
 * @return {"code":0,"msg":"success","data":{"permission_list":[{"id":1,"type":1,"name":"系统","frontend_url":"sysSetting","open":true,"children":[{"id":2,"parent_id":1,"type":1,"name":"全局设置","backend_url":"basic-setting","frontend_url":"sysSetting/basicSetting","open":true,"children":[{"id":3,"parent_id":2,"type":1,"name":"基本信息","backend_url":"user-basic-info","frontend_url":"sysSetting/basicSetting/sysBasicSet","open":true,"children":[{"id":4,"parent_id":3,"type":2,"name":"保存","backend_url":"update-basic-setting","frontend_url":"update-basic-setting"}]},{"id":5,"parent_id":2,"type":1,"name":"会员注册","backend_url":"register-setting","frontend_url":"sysSetting/basicSetting/sysMemReg","open":true,"children":[{"id":6,"parent_id":5,"type":2,"name":"保存","backend_url":"update-register-setting","frontend_url":"update-register-setting"}]},{"id":7,"parent_id":2,"type":1,"name":"积分设置","backend_url":"points-setting","frontend_url":"sysSetting/basicSetting/sysMemScore","open":true,"children":[{"id":8,"parent_id":7,"type":2,"name":"保存","backend_url":"update-points-setting","frontend_url":"update-points-setting"}]},{"id":175,"parent_id":2,"type":2,"name":"签到配置","backend_url":"sign-setting","frontend_url":"sysSetting/sysMemScore","children":[{"id":176,"parent_id":175,"type":1,"name":"开关","backend_url":"sign-switch","frontend_url":"sysSetting/sysMemScore","open":true},{"id":177,"parent_id":175,"type":1,"name":"保存","backend_url":"update-sign-setting","frontend_url":"sysSetting/sysMemScore","open":true}]}]},{"id":155,"parent_id":1,"type":1,"name":"风险控制","backend_url":"risk-forewarn-log","frontend_url":"sysSetting/sysRiskforewarn","open":true,"children":[{"id":156,"parent_id":155,"type":1,"name":"事件记录","backend_url":"risk-forewarn-log","frontend_url":"sysSetting/sysRiskforewarn/riskForewarnLog","open":true},{"id":157,"parent_id":155,"type":1,"name":"预警策略","backend_url":"risk-forewarn-setting","frontend_url":"sysSetting/sysRiskforewarn/riskForewarnSetting","open":true},{"id":160,"parent_id":155,"type":2,"name":"获取风险预警配置","backend_url":"get-risk-forewarn-config","frontend_url":"get-risk-forewarn-config"}]},{"id":16,"parent_id":1,"type":1,"name":"游戏管理","backend_url":"games","frontend_url":"sysSetting/sysGame","open":true,"children":[{"id":17,"parent_id":16,"type":2,"name":"修改游戏","backend_url":"update-games","frontend_url":"update-games"}]},{"id":18,"parent_id":1,"type":1,"name":"域名绑定","backend_url":"domains","frontend_url":"sysSetting/sysDomain","open":true,"children":[{"id":19,"parent_id":18,"type":2,"name":"修改站点","backend_url":"update-domains","frontend_url":"update-domains"}]},{"id":20,"parent_id":1,"type":1,"name":"屏蔽IP地区","backend_url":"site-deny","frontend_url":"sysSetting/sysShield","open":true,"children":[{"id":21,"parent_id":20,"type":2,"name":"添加屏蔽IP","backend_url":"add-deny-ip","frontend_url":"add-deny-ip"},{"id":22,"parent_id":20,"type":2,"name":"添加屏蔽地区","backend_url":"add-deny-address","frontend_url":"add-deny-address"},{"id":23,"parent_id":20,"type":2,"name":"删除屏蔽地区","backend_url":"delete-deny","frontend_url":"delete-deny"}]},{"id":24,"parent_id":1,"type":1,"name":"员工账号","backend_url":"admins","frontend_url":"sysSetting/sysAccount","open":true,"children":[{"id":25,"parent_id":24,"type":2,"name":"添加员工","backend_url":"create-admin","frontend_url":"create-admin"},{"id":26,"parent_id":24,"type":2,"name":"编辑员工","backend_url":"update-admin","frontend_url":"update-admin"},{"id":27,"parent_id":24,"type":2,"name":"删除员工","backend_url":"delete-admin","frontend_url":"delete-admin"},{"id":28,"parent_id":24,"type":2,"name":"职务列表","backend_url":"roles","frontend_url":"sysSetting/sysAccount/roles"},{"id":29,"parent_id":24,"type":1,"name":"职务权限","backend_url":"permissions","frontend_url":"sysSetting/sysAccount/permissions","open":true},{"id":30,"parent_id":24,"type":2,"name":"添加职务","backend_url":"create-role","frontend_url":"create-role"},{"id":31,"parent_id":24,"type":2,"name":"编辑职务","backend_url":"update-role","frontend_url":"update-role"},{"id":32,"parent_id":24,"type":2,"name":"删除职务","backend_url":"delete-role","frontend_url":"delete-role"},{"id":33,"parent_id":24,"type":2,"name":"保存职务","backend_url":"save-permission","frontend_url":"save-permission"},{"id":159,"parent_id":24,"type":2,"name":"二次验证","backend_url":"bind-google2fa"},{"id":185,"parent_id":24,"type":1,"name":"员工列表","backend_url":"admins","frontend_url":"sysSetting/sysAccount/admins","open":true}]},{"id":34,"parent_id":1,"type":1,"name":"后台操作日志","backend_url":"admin-logs","frontend_url":"sysSetting/sysJournal","open":true},{"id":35,"parent_id":1,"type":2,"name":"自定义标签内容","children":[{"id":36,"parent_id":35,"type":2,"name":"标签列表","backend_url":"custom-tag","frontend_url":"custom-tag"},{"id":37,"parent_id":35,"type":2,"name":"添加标签","backend_url":"add-custom-tag","frontend_url":"add-custom-tag"},{"id":38,"parent_id":35,"type":2,"name":"编辑标签","backend_url":"update-custom-tag","frontend_url":"update-custom-tag"},{"id":39,"parent_id":35,"type":2,"name":"保存标签","backend_url":"update-custom-tag","frontend_url":"update-custom-tag"},{"id":40,"parent_id":35,"type":2,"name":"删除标签","backend_url":"delete-custom-tag","frontend_url":"delete-custom-tag"}]}]},{"id":41,"type":1,"name":"运营","frontend_url":"opeReport","open":true,"children":[{"id":42,"parent_id":41,"type":1,"name":"全局报表","backend_url":"global-report","frontend_url":"opeReport","open":true},{"id":43,"parent_id":41,"type":1,"name":"活动管理","backend_url":"activities","frontend_url":"opeActMange","open":true,"children":[{"id":44,"parent_id":43,"type":2,"name":"添加活动","backend_url":"create-activity","frontend_url":"create-activity"},{"id":45,"parent_id":43,"type":2,"name":"编辑活动","backend_url":"update-activity","frontend_url":"update-activity"},{"id":46,"parent_id":43,"type":2,"name":"保存活动","backend_url":"update-activity","frontend_url":"update-activity"},{"id":47,"parent_id":43,"type":2,"name":"发布活动","backend_url":"publish-activity","frontend_url":"publish-activity"},{"id":48,"parent_id":43,"type":2,"name":"上传活动图片","backend_url":"upload-activity-pic","frontend_url":"upload-activity-pic"},{"id":49,"parent_id":43,"type":2,"name":"活动排序","backend_url":"activity-sort","frontend_url":"activity-sort"},{"id":50,"parent_id":43,"type":2,"name":"活动模块列表","backend_url":"activity-module","frontend_url":"activity-module"},{"id":51,"parent_id":43,"type":2,"name":"活动模块记录","backend_url":"activity-module-logs","frontend_url":"activity-module-logs"},{"id":52,"parent_id":43,"type":2,"name":"编辑活动模块","backend_url":"update-activity-module","frontend_url":"update-activity-module"},{"id":53,"parent_id":43,"type":2,"name":"保存活动模块","backend_url":"update-activity-module","frontend_url":"update-activity-module"},{"id":54,"parent_id":43,"type":2,"name":"删除活动模块","backend_url":"delete-activity-module","frontend_url":"delete-activity-module"},{"id":55,"parent_id":43,"type":2,"name":"获取活动模块列表","backend_url":"activity-template","frontend_url":"activity-template"},{"id":56,"parent_id":43,"type":2,"name":"添加充值模块","backend_url":"create-activity-recharge","frontend_url":"create-activity-recharge"},{"id":57,"parent_id":43,"type":2,"name":"编辑充值模块","backend_url":"update-activity-recharge","frontend_url":"update-activity-recharge"},{"id":58,"parent_id":43,"type":2,"name":"保存充值模块","backend_url":"activity-recharge-logs","frontend_url":"activity-recharge-logs"},{"id":59,"parent_id":43,"type":2,"name":"添加大转盘模块","backend_url":"create-activity-roulette","frontend_url":"create-activity-roulette"},{"id":60,"parent_id":43,"type":2,"name":"添加开宝箱模块","backend_url":"create-activity-box","frontend_url":"create-activity-box"},{"id":61,"parent_id":43,"type":2,"name":"添加砸金蛋模块","backend_url":"create-activity-egg","frontend_url":"create-activity-egg"},{"id":62,"parent_id":43,"type":2,"name":"添加抢红包模块","backend_url":"create-activity-red-packet","frontend_url":"create-activity-red-packet"}]},{"id":63,"parent_id":41,"type":1,"name":"广告管理","backend_url":"ads","frontend_url":"opeBomb","open":true,"children":[{"id":64,"parent_id":63,"type":2,"name":"添加广告","backend_url":"create-ad","frontend_url":"create"},{"id":65,"parent_id":63,"type":2,"name":"编辑广告","backend_url":"update-ad","frontend_url":"update"},{"id":66,"parent_id":63,"type":2,"name":"删除广告","backend_url":"delete-ad","frontend_url":"destroy"}]},{"id":67,"parent_id":41,"type":1,"name":"公告管理","backend_url":"notices","frontend_url":"opeNotice","open":true,"children":[{"id":68,"parent_id":67,"type":2,"name":"添加公告","backend_url":"create-notice","frontend_url":"create-notice"},{"id":69,"parent_id":67,"type":2,"name":"编辑公告","backend_url":"update-notice","frontend_url":"update-notice"},{"id":70,"parent_id":67,"type":2,"name":"删除公告","backend_url":"delete-notice","frontend_url":"delete-notice"}]},{"id":71,"parent_id":41,"type":1,"name":"消息管理","backend_url":"messages","frontend_url":"opeMessage","open":true,"children":[{"id":72,"parent_id":71,"type":2,"name":"添加消息","backend_url":"create-message","frontend_url":"create-message"},{"id":169,"parent_id":71,"type":2,"name":"添加代理消息","backend_url":"create-agent-message","frontend_url":"create-agent-message"},{"id":170,"parent_id":71,"type":2,"name":"代理消息列表","backend_url":"agent-messages","frontend_url":"agent-messages"}]}]},{"id":73,"type":1,"name":"财务","frontend_url":"finance","open":true,"children":[{"id":74,"parent_id":73,"type":1,"name":"账变记录","backend_url":"balance-changes","frontend_url":"finance","open":true},{"id":75,"parent_id":73,"type":1,"name":"充值记录","backend_url":"recharges","frontend_url":"finRecharge({fastclick:2})","open":true,"children":[{"id":76,"parent_id":75,"type":2,"name":"在线支付","backend_url":"recharge-payment","frontend_url":"finRecharge/finRecOnline"},{"id":77,"parent_id":75,"type":2,"name":"转账汇款","backend_url":"recharge-transfer","frontend_url":"finRecharge/finRecTran"},{"id":78,"parent_id":75,"type":2,"name":"后台加款","backend_url":"recharge-manual","frontend_url":"finRecharge/finRecAdd"},{"id":79,"parent_id":75,"type":2,"name":"充值详情","backend_url":"recharge-review","frontend_url":"recharge-review"},{"id":80,"parent_id":75,"type":2,"name":"审核充值","backend_url":"deal-with-recharge","frontend_url":"deal-with-recharge"},{"id":81,"parent_id":75,"type":2,"name":"解锁审核","backend_url":"recharge-review-unlock","frontend_url":"recharge-review-unlock"},{"id":154,"parent_id":75,"type":2,"name":"确认支付订单","backend_url":"confirm-payment-order","frontend_url":"finRecharge/finRecOnline"}]},{"id":82,"parent_id":73,"type":1,"name":"提现记录","backend_url":"withdraws","frontend_url":"withdrawalsRecord","open":true,"children":[{"id":83,"parent_id":82,"type":2,"name":"提款详情","backend_url":"withdraw-review","frontend_url":"withdraw-review"},{"id":84,"parent_id":82,"type":2,"name":"审核提款","backend_url":"deal-with-withdraw","frontend_url":"deal-with-withdraw"},{"id":85,"parent_id":82,"type":2,"name":"解锁审核","backend_url":"withdraw-review-unlock","frontend_url":"withdraw-review-unlock"},{"id":86,"parent_id":82,"type":2,"name":"会员报表","backend_url":"user-report","frontend_url":"user-report"},{"id":158,"parent_id":82,"type":2,"name":"后台扣款","backend_url":"withdraw-manual","frontend_url":"withdrawalsRecord/withdrawalsRecAdd"}]},{"id":165,"parent_id":73,"type":1,"name":"游戏转账","backend_url":"game-transfer-logs","frontend_url":"gameTransfer","open":true,"children":[{"id":166,"parent_id":165,"type":2,"name":"游戏转账记录","backend_url":"game-transfer-logs","frontend_url":"gameTransfer"}]},{"id":87,"parent_id":73,"type":1,"name":"加扣款","frontend_url":"finDebit","open":true,"children":[{"id":88,"parent_id":87,"type":2,"name":"查询会员账户余额","backend_url":"query-user-balance","frontend_url":"query-user-balance"},{"id":89,"parent_id":87,"type":2,"name":"查询第三方账户余额","backend_url":"query-game-balance","frontend_url":"query-game-balance"},{"id":90,"parent_id":87,"type":2,"name":"手动操作中心账户","backend_url":"manual-user-balance","frontend_url":"manual-user-balance"},{"id":91,"parent_id":87,"type":2,"name":"手动操作第三方账户","backend_url":"manual-game-balance","frontend_url":"manual-game-balance"}]},{"id":92,"parent_id":73,"type":1,"name":"接口设置","backend_url":"payment-accounts","frontend_url":"finInterface","open":true,"children":[{"id":93,"parent_id":92,"type":2,"name":"添加支付接口","backend_url":"create-payment-account","frontend_url":"create-payment-account"},{"id":94,"parent_id":92,"type":2,"name":"编辑接口","backend_url":"update-payment-account","frontend_url":"update-payment-account"},{"id":95,"parent_id":92,"type":2,"name":"保存接口","backend_url":"update-payment-account","frontend_url":"update-payment-account"},{"id":96,"parent_id":92,"type":2,"name":"删除支付接口","backend_url":"delete-payment-account","frontend_url":"delete-payment-account"},{"id":97,"parent_id":92,"type":2,"name":"银行列表","backend_url":"options-bank","frontend_url":"options-bank"},{"id":98,"parent_id":92,"type":2,"name":"转账汇款","backend_url":"transfer-accounts","frontend_url":"transfer-accounts"},{"id":99,"parent_id":92,"type":2,"name":"添加转账汇款","backend_url":"create-transfer-account","frontend_url":"create-transfer-account"},{"id":100,"parent_id":92,"type":2,"name":"编辑转账汇款","backend_url":"update-transfer-account","frontend_url":"update-transfer-account"},{"id":101,"parent_id":92,"type":2,"name":"保存转账汇款","backend_url":"update-transfer-account","frontend_url":"update-transfer-account"},{"id":102,"parent_id":92,"type":2,"name":"删除转账汇款","backend_url":"delete-transfer-account","frontend_url":"delete-transfer-account"}]},{"id":103,"parent_id":73,"type":1,"name":"财务报表","backend_url":"finance-report","frontend_url":"finReport","open":true}]},{"id":104,"type":1,"name":"会员","frontend_url":"memberLists","open":true,"children":[{"id":105,"parent_id":104,"type":1,"name":"会员列表","backend_url":"users","frontend_url":"memberList/memberList","open":true,"children":[{"id":106,"parent_id":105,"type":2,"name":"编辑","backend_url":"update-user","frontend_url":"update-user"},{"id":107,"parent_id":105,"type":2,"name":"基本信息","backend_url":"user-basic-info","frontend_url":"user-basic-info"},{"id":108,"parent_id":105,"type":2,"name":"会员报表","backend_url":"user-report","frontend_url":"user-report"},{"id":153,"parent_id":105,"type":2,"name":"积分列表","backend_url":"user-points","frontend_url":"memberList"},{"id":161,"parent_id":105,"type":2,"name":"会员详细信息","backend_url":"user-details-info","frontend_url":"user-details-info"},{"id":162,"parent_id":105,"type":2,"name":"获取会员统计信息","backend_url":"user-statistics-info","frontend_url":"user-statistics-info"},{"id":172,"parent_id":105,"type":2,"name":"获取自定义字段","backend_url":"get-fields-for-user-page"},{"id":173,"parent_id":105,"type":2,"name":"保存自定义字段","backend_url":"save-fields-for-user-page"}]},{"id":109,"parent_id":104,"type":1,"name":"会员返水","backend_url":"rebate-unconfirmed","frontend_url":"memberList/memberFanShui","open":true,"children":[{"id":110,"parent_id":109,"type":2,"name":"会员返水确认","backend_url":"confirm-rebate","frontend_url":"confirm-rebate"},{"id":111,"parent_id":109,"type":2,"name":"生成返水","backend_url":"generate-rebate","frontend_url":"generate-rebate"},{"id":112,"parent_id":109,"type":2,"name":"返水历史","backend_url":"rebate-history","frontend_url":"rebate-history"},{"id":113,"parent_id":109,"type":2,"name":"返水详情","backend_url":"rebate-detail","frontend_url":"rebate-detail"},{"id":9,"parent_id":109,"type":1,"name":"返水设置","backend_url":"rebate-setting","frontend_url":"memberList/memberFanShui/memfssetting","open":true,"children":[{"id":10,"parent_id":9,"type":2,"name":"添加规则","backend_url":"create-rebate-rule","frontend_url":"create-rebate-rule"},{"id":11,"parent_id":9,"type":2,"name":"编辑规则","backend_url":"update-rebate-rule","frontend_url":"update-rebate-rule"},{"id":12,"parent_id":9,"type":2,"name":"删除规则","backend_url":"delete-rebate-rule","frontend_url":"delete-rebate-rule"},{"id":13,"parent_id":9,"type":2,"name":"添加条件","backend_url":"create-rebate-option","frontend_url":"create-rebate-option"},{"id":14,"parent_id":9,"type":2,"name":"编辑条件","backend_url":"update-rebate-option","frontend_url":"update-rebate-option"},{"id":15,"parent_id":9,"type":2,"name":"删除条件","backend_url":"delete-rebate-option","frontend_url":"delete-rebate-option"}]}]},{"id":114,"parent_id":104,"type":1,"name":"会员等级","backend_url":"user-grades","frontend_url":"memberList/memberGrade","open":true,"children":[{"id":115,"parent_id":114,"type":2,"name":"保存会员等级","backend_url":"save-user-grades","frontend_url":"save-user-grades"},{"id":116,"parent_id":114,"type":2,"name":"删除会员等级","backend_url":"delete-user-grades","frontend_url":"delete-user-grades"}]},{"id":117,"parent_id":104,"type":1,"name":"会员层级","backend_url":"user-levels","frontend_url":"memberList/memberLevel","open":true,"children":[{"id":118,"parent_id":117,"type":2,"name":"新增会员层级","backend_url":"create-user-level","frontend_url":"create-user-level"},{"id":119,"parent_id":117,"type":2,"name":"编辑会员层级","backend_url":"update-user-level","frontend_url":"update-user-level"},{"id":120,"parent_id":117,"type":2,"name":"删除会员层级","backend_url":"delete-user-level","frontend_url":"delete-user-level"}]},{"id":121,"parent_id":104,"type":1,"name":"会员登录日志","backend_url":"user-login-logs","frontend_url":"memberLoginLog","open":true},{"id":163,"parent_id":104,"type":1,"name":"站内信","open":true,"children":[{"id":164,"parent_id":163,"type":2,"name":"会员站内信息统计","backend_url":"user-message-statistic","frontend_url":"user-message-statistic"}]}]},{"id":122,"type":1,"name":"代理","frontend_url":"agent","open":true,"children":[{"id":123,"parent_id":122,"type":1,"name":"代理列表","backend_url":"agents","frontend_url":"agent","open":true,"children":[{"id":124,"parent_id":123,"type":2,"name":"基本信息","backend_url":"agent-basic","frontend_url":"agent-basic"},{"id":125,"parent_id":123,"type":2,"name":"代理报表","backend_url":"agent-report","frontend_url":"agent-report"},{"id":126,"parent_id":123,"type":2,"name":"编辑代理","backend_url":"update-agent","frontend_url":"update-agent"},{"id":127,"parent_id":123,"type":2,"name":"保存代理","backend_url":"update-agent","frontend_url":"update-agent"},{"id":128,"parent_id":123,"type":2,"name":"代理域名列表","backend_url":"agent-domain","frontend_url":"agent-domain"},{"id":129,"parent_id":123,"type":2,"name":"添加代理域名","backend_url":"add-agent-domain","frontend_url":"add-agent-domain"},{"id":130,"parent_id":123,"type":2,"name":"编辑代理域名","backend_url":"update-agent-domain","frontend_url":"update-agent-domain"},{"id":131,"parent_id":123,"type":2,"name":"保存代理域名","backend_url":"update-agent-domain","frontend_url":"update-agent-domain"},{"id":132,"parent_id":123,"type":2,"name":"删除代理域名","backend_url":"delete-agent-domain","frontend_url":"delete-agent-domain"},{"id":167,"parent_id":123,"type":2,"name":"下级会员","backend_url":"agent-users"},{"id":168,"parent_id":123,"type":2,"name":"游戏报表","backend_url":"agent-game-report"},{"id":171,"parent_id":123,"type":2,"name":"代理等级列表","backend_url":"agent-level","frontend_url":"agent-level"}]},{"id":133,"parent_id":122,"type":1,"name":"佣金结算","backend_url":"agent-sharing-report","frontend_url":"agentComLog","open":true,"children":[{"id":134,"parent_id":133,"type":2,"name":"生成报表","backend_url":"generate-agent-sharing","frontend_url":"generate-agent-sharing"},{"id":135,"parent_id":133,"type":2,"name":"返佣详情","backend_url":"sharing-review","frontend_url":"sharing-review"},{"id":136,"parent_id":133,"type":2,"name":"解除锁定","backend_url":"sharing-review-unlock","frontend_url":"sharing-review-unlock"},{"id":137,"parent_id":133,"type":2,"name":"确定返佣","backend_url":"deal-with-sharing","frontend_url":"deal-with-sharing"},{"id":138,"parent_id":133,"type":2,"name":"代理报表","backend_url":"agent-report","frontend_url":"agent-report"},{"id":139,"parent_id":133,"type":2,"name":"返佣历史","backend_url":"agent-sharing-history","frontend_url":"agent-sharing-history"}]},{"id":140,"parent_id":122,"type":1,"name":"代理审核","backend_url":"pending-agents","frontend_url":"agentCheck({fastclick:2})","open":true,"children":[{"id":141,"parent_id":140,"type":2,"name":"审核代理","backend_url":"check-agent","frontend_url":"check-agent"}]},{"id":142,"parent_id":122,"type":1,"name":"代理层级","backend_url":"agent-levels","frontend_url":"agentLevel","open":true,"children":[{"id":143,"parent_id":142,"type":2,"name":"保存代理层级","backend_url":"save-agent-level","frontend_url":"save-agent-level"},{"id":144,"parent_id":142,"type":2,"name":"删除代理层级","backend_url":"delete-agent-level","frontend_url":"delete-agent-level"}]}]},{"id":145,"type":1,"name":"游戏","frontend_url":"game","open":true,"children":[{"id":146,"parent_id":145,"type":1,"name":"游戏报表","backend_url":"game-all-report","frontend_url":"game","open":true},{"id":147,"parent_id":145,"type":1,"name":"体育记录","backend_url":"game-record","frontend_url":"gameSportRecord","open":true,"children":[{"id":152,"parent_id":147,"type":2,"name":"体育串关详情","backend_url":"sports-parlay-detail"}]},{"id":148,"parent_id":145,"type":1,"name":"彩票记录","backend_url":"game-record","frontend_url":"gameLotteryRecord","open":true},{"id":149,"parent_id":145,"type":1,"name":"视讯记录","backend_url":"game-record","frontend_url":"gameAGRecord","open":true},{"id":150,"parent_id":145,"type":1,"name":"电子游戏","backend_url":"game-record","frontend_url":"gameVideoRecord","open":true},{"id":151,"parent_id":145,"type":1,"name":"游戏分析","backend_url":"game-all-situation","frontend_url":"gameAnalysis","open":true},{"id":174,"parent_id":145,"type":1,"name":"棋牌游戏","backend_url":"game-record","frontend_url":"gamePokerRecord","open":true}]},{"id":178,"type":1,"name":"文档管理","frontend_url":"documentation","open":true,"children":[{"id":179,"parent_id":178,"type":1,"name":"文档","backend_url":"index","frontend_url":"documentation/index","open":true}]},{"id":180,"type":1,"name":"综合示例","frontend_url":"example","open":true,"children":[{"id":181,"parent_id":180,"type":1,"name":"创建文章","frontend_url":"example/create","open":true},{"id":182,"parent_id":180,"type":1,"name":"编辑文章","frontend_url":"example/edit","open":true},{"id":183,"parent_id":180,"type":1,"name":"文章列表","frontend_url":"example/list","open":true}]}],"role_list":[{"id":1,"name":"管理员","permission_list":["1","2","3","4","5","6","7","8","175","176","177","155","156","157","160","16","17","18","19","20","21","22","23","24","25","26","27","28","29","30","31","32","33","159","185","34","35","36","37","38","39","40","41","43","44","45","46","47","48","49","50","51","52","53","54","55","56","57","58","59","60","61","62","63","64","65","66","67","68","69","70","71","72","169","170","73","75","76","77","78","79","80","81","154","82","83","84","85","86","158","165","166","87","88","89","90","91","92","93","94","95","96","97","98","99","100","101","102","103","105","106","107","108","161","162","172","173","109","110","111","112","113","9","10","11","12","13","14","15","114","115","116","117","118","119","120","121","163","164","122","124","125","126","127","128","129","130","131","132","167","168","171","133","134","135","136","137","138","139","140","141","142","143","144","145","147","152","148","149","150","151","174"]},{"id":2,"name":"客服","permission_list":["1","16","17","73","74","75","76","77","78","79","80","81","154","87","88","89","90","91"]},{"id":3,"name":"财务","permission_list":["1","2","4","5","6","7","8","9","10","11","12","13","14","15","16","17","18","19","20","21","22","23","24","25","26","27","28","29","30","31","32","33","34","35","36","37","38","39","40","73","74","75","76","77","78","79","80","81","82","83","84","85","86","87","88","89","90","91","92","93","94","95","96","97","98","99","100","101","102","103","104","105","106","107","108","109","110","111","112","113","114","115","116","117","118","119","120","121","122","133","134","135"]},{"id":4,"name":"运营","permission_list":["1","2","4","5","6","7","8","9","10","11","12","13","14","15","16","17","18","19","20","21","22","23","24","25","26","27","28","29","30","31","32","33","41","42","43","44","45","46","47","48","49","50","51","52","53","54","55","56","57","58","59","60","61","62","63","64","65","66","67","68","69","70","71","72","73","74","75","76","77","78","79","80","81","82","83","84","85","86","87","88","89","90","91","92","93","94","95","96","97","98","99","100","101","102","103","104","105","106","107","108","109","110","111","112","113","114","115","116","117","118","119","120","121","122","123","124","125","126","127","128","129","130","131","132","133","134","135","136","137","138","139","140","141","142","143","144","145","146","147","152","148","149","150","151"]},{"id":6,"name":"测试","permission_list":["1","2","3"]},{"id":21,"name":"这个太好","permission_list":["1","2","3","4","5","6","7","8","175","176","177","155","156","157","160","16","17","18","19","20","21","22","23","24","25","26","27","28","29","30","31","32","33","159","185","34","35","36","37","38","39","40","41","42","43","44","45","46","47","48","49","50","51","52","53","54","55","56","57","58","59","60","61","62","63","64","65","66","67","68","69","70","71","72","169","170","73","74","75","76","77","78","79","80","81","154","82","83","84","85","86","158","165","166","87","88","89","90","91","92","93","94","95","96","97","98","99","100","101","102","103","104","105","106","107","108","153","161","162","172","173","109","110","111","112","113","9","10","11","12","13","14","15","114","115","116","117","118","119","120","121","163","164","122","123","124","125","126","127","128","129","130","131","132","167","168","171","133","134","135","136","137","138","139","140","141","142","143","144","145","146","147","152","148","149","150","151","174","178","179","180","181","182","183"]},{"id":22,"name":"tester111"},{"id":23,"name":"tester121"}]}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.permission_list array 权限树列表
 * @return_param data.role_list array 角色权限列表
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callGetPermissions(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Role GetPermissions")

	siteId := int32(1) // 默认站点ID
	if s := r.Get("site_id"); s != nil {
		if siteIdInt := s.Int32(); siteIdInt > 0 {
			siteId = siteIdInt
		}
	}

	// 创建 gRPC 客户端
	client := v2.NewRoleClient(conn)
	req := &v2.GetPermissionsReq{
		SiteId: siteId,
	}

	// 调用 gRPC 服务
	res, err := client.GetPermissions(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取权限列表失败，请稍后重试")
		return nil
	}

	// 添加调试日志检查gRPC响应数据
	util.LogWithTrace(ctx, "debug", "gRPC response - permission count: %d, role count: %d",
		len(res.PermissionList), len(res.RoleList))

	if len(res.PermissionList) > 0 {
		firstPerm := res.PermissionList[0]
		util.LogWithTrace(ctx, "debug", "first permission - ID: %d, Name: %s, Children count: %d, Children is nil: %v",
			firstPerm.Id, firstPerm.Name, len(firstPerm.Children), firstPerm.Children == nil)

		if len(firstPerm.Children) > 0 {
			firstChild := firstPerm.Children[0]
			util.LogWithTrace(ctx, "debug", "first child permission - ID: %d, Name: %s, Children count: %d, Children is nil: %v",
				firstChild.Id, firstChild.Name, len(firstChild.Children), firstChild.Children == nil)
		}
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 保存角色权限
 * @description 保存角色权限
 * @method post
 * @url /api/backend/app/save-permission
 * @param token 必选 string 员工token
 * @param id 必选 int 角色ID
 * @param permission_list 必选 string 权限ID列表，逗号分隔
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"保存成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @return_param msg string 提示说明
 * @remark 备注
 * @number 1
 */
func callSavePermission(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Role SavePermission")

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

	permissionList := ""
	if p, ok := reqData["permission_list"].(string); ok {
		permissionList = p
	}

	siteId := int32(1) // 默认站点ID
	if s, ok := reqData["site_id"].(float64); ok {
		siteId = int32(s)
	}

	// 创建 gRPC 客户端
	client := v2.NewRoleClient(conn)
	req := &v2.SavePermissionReq{
		Id:             id,
		SiteId:         siteId,
		PermissionList: permissionList,
	}

	// 调用 gRPC 服务
	res, err := client.SavePermission(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "保存权限失败，请稍后重试")
		return nil
	}

	// 根据业务逻辑返回响应
	if !res.Success {
		util.WriteBadRequest(r, res.Message)
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台
 * @title 退出登录
 * @description 管理员退出登录的接口
 * @method post
 * @url /api/backend/app/logout
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 管理员修改密码
 * @description 管理员修改密码
 * @method post
 * @url /api/backend/app/change-password
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 获取员工列表
 * @description 获取员工列表
 * @method get
 * @url /api/backend/app/admins
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

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/系统/员工账号
 * @title 编辑员工
 * @description 编辑员工信息
 * @method post
 * @url /api/backend/app/update-admin
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
 * @url /api/backend/app/delete-admin
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

/**
 * showdoc
 * @catalog 后台/系统/文件上传
 * @title 上传图片
 * @description 上传图片文件的接口
 * @method post
 * @url /api/backend/app/upload-image
 * @param image 必选 file 图片文件
 * @param code 可选 string 上传标识 (default, mobile_logo)
 * @return {"code":0,"msg":"success","data":{"image":"http://localhost:19000/uploads/site_1/2025/12/电子游戏_1767127541.jpeg"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.image string 图片访问URL
 * @return_param msg string 提示说明
 * @remark 支持jpg、jpeg、gif、png格式，默认最大500KB，mobile_logo类型支持svg格式最大100KB
 * @number 10
 */
func callUploadImage(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Upload UploadImage")

	// 检查请求的 Content-Type 是否为 multipart/form-data
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		util.WriteBadRequest(r, "请使用 multipart/form-data 格式上传文件")
		return nil
	}

	// 获取上传的文件
	file := r.GetUploadFile("image")
	if file == nil {
		util.WriteBadRequest(r, "请选择要上传的图片")
		return nil
	}

	// 获取上传标识
	uploadCode := r.Get("code", "default").String()

	// 验证文件类型 - 通过文件扩展名和Content-Type判断
	fileContentType := file.Header.Get("Content-Type")
	if fileContentType == "" {
		// 如果没有 Content-Type，尝试从文件扩展名推断
		ext := strings.ToLower(filepath.Ext(file.Filename))
		switch ext {
		case ".jpg", ".jpeg":
			fileContentType = "image/jpeg"
		case ".png":
			fileContentType = "image/png"
		case ".gif":
			fileContentType = "image/gif"
		case ".svg":
			fileContentType = "image/svg+xml"
		default:
			util.WriteBadRequest(r, "不支持的文件格式")
			return nil
		}
	}

	if !strings.HasPrefix(fileContentType, "image/") {
		util.WriteBadRequest(r, "请上传图片文件")
		return nil
	}

	// 读取文件内容
	fileReader, err := file.Open()
	if err != nil {
		util.WriteInternalError(r, "读取文件失败")
		return nil
	}
	defer fileReader.Close()

	fileData, err := io.ReadAll(fileReader)
	if err != nil {
		util.WriteInternalError(r, "读取文件内容失败")
		return nil
	}

	// 验证文件大小
	if len(fileData) == 0 {
		util.WriteBadRequest(r, "文件内容为空")
		return nil
	}

	// 验证文件大小限制 (在这里做基本检查，详细检查在服务端)
	maxSize := int64(5 * 1024 * 1024) // 5MB 硬限制
	if int64(len(fileData)) > maxSize {
		util.WriteBadRequest(r, "文件大小不能超过5MB")
		return nil
	}

	util.LogWithTrace(ctx, "info", "上传文件信息 - 文件名: %s, 大小: %d, 类型: %s, 上传标识: %s",
		file.Filename, len(fileData), fileContentType, uploadCode)

	// 创建 gRPC 客户端
	client := v4.NewUploadClient(conn)
	req := &v4.UploadImageReq{
		FileData:    fileData,
		FileName:    file.Filename,
		ContentType: fileContentType,
		FileSize:    int64(len(fileData)),
		UploadCode:  uploadCode,
	}

	// 调用 gRPC 服务
	res, err := client.UploadImage(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		errorMsg := err.Error()
		util.LogWithTrace(ctx, "error", "gRPC上传失败: %v", err)

		if strings.Contains(errorMsg, "文件类型") || strings.Contains(errorMsg, "不支持") {
			util.WriteBadRequest(r, "不支持的文件类型")
		} else if strings.Contains(errorMsg, "文件大小") || strings.Contains(errorMsg, "超过") {
			util.WriteBadRequest(r, "文件大小超出限制")
		} else if strings.Contains(errorMsg, "文件名") || strings.Contains(errorMsg, "为空") {
			util.WriteBadRequest(r, "文件参数错误")
		} else if strings.Contains(errorMsg, "上传失败") || strings.Contains(errorMsg, "MinIO") {
			util.WriteInternalError(r, "文件上传失败，请稍后重试")
		} else {
			util.WriteInternalError(r, "上传服务暂时不可用，请稍后重试")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"image": res.ImageUrl,
	})

	util.LogWithTrace(ctx, "info", "文件上传成功 - URL: %s", res.ImageUrl)
	return nil
}

/**
 * showdoc
 * @catalog 后台/系统/后台操作日志
 * @title 后台操作日志
 * @description 后台操作日志
 * @method get
 * @url /api/backend/app/admin-logs
 * @param username 可选 string 用户名筛选
 * @param start 可选 string 开始时间 (格式: 2023-01-01 00:00:00)
 * @param end 可选 string 结束时间 (格式: 2023-01-31 23:59:59)
 * @param page 可选 int 页码 (默认: 1)
 * @param size 可选 int 每页数量 (默认: 50)
 * @return {"code":0,"msg":"success","data":{"list":[{"username":"admin","ip":"127.0.0.1","remark":"登录成功","created_at":"2023-01-01 12:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.list array 日志列表
 * @return_param data.list.username string 管理员用户名
 * @return_param data.list.ip string IP地址
 * @return_param data.list.remark string 操作备注
 * @return_param data.list.created_at string 创建时间
 * @return_param data.count int 总数量
 * @return_param msg string 提示说明
 * @remark 支持按用户名和时间范围筛选，支持分页查询
 * @number 11
 */
func callGetAdminLogs(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Admin GetAdminLogs - Method: %s", r.Method)

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.LogWithTrace(ctx, "error", "请求参数解析失败: %v", err)
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	util.LogWithTrace(ctx, "info", "解析到的请求数据: %+v", reqData)

	// 创建 gRPC 客户端
	client := v1.NewAdminClient(conn)
	req := &v1.GetAdminLogsReq{
		Username: getStringFromMap(reqData, "username"),
		Start:    getStringFromMap(reqData, "start"),
		End:      getStringFromMap(reqData, "end"),
		Page:     getInt32FromMap(reqData, "page"),
		Size:     getInt32FromMap(reqData, "size"),
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Username: %s, Start: %s, End: %s, Page: %d, Size: %d",
		req.Username, req.Start, req.End, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetAdminLogs(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取管理员日志失败，请稍后重试")
		return nil
	}

	util.LogWithTrace(ctx, "info", "gRPC响应成功，数据条数: %d", len(res.List))

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/会员/会员列表
 * @title 获取会员列表
 * @description 获取会员列表的接口
 * @method get
 * @url /api/backend/app/users
 * @param grade_id 可选 int 等级ID
 * @param level_id 可选 int 层级ID
 * @param status 可选 int 状态
 * @param username 可选 string 用户名
 * @param realname 可选 string 真实姓名
 * @param agent_username 可选 string 代理用户名
 * @param mobile 可选 string 手机号
 * @param page 可选 int 页码 (默认: 1)
 * @param size 可选 int 每页数量 (默认: 20)
 * @param sort_field 可选 string 排序字段
 * @param sort_rule 可选 int 排序规则 1=ASC, 0=DESC
 * @param card_no 可选 string 银行卡号
 * @param domain 可选 string 注册域名
 * @param start_date 可选 string 开始日期
 * @param end_date 可选 string 结束日期
 * @param charge 可选 int 是否首存 1=是, 0=否
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"username":"test","grade_name":"普通会员","level_name":"一级","agent_username":"agent1","status":1}],"count":1,"total_users":100,"total_charge_users":50}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.list array 用户列表
 * @return_param data.count int 总数量
 * @return_param data.total_users int 总用户数
 * @return_param data.total_charge_users int 总充值用户数
 * @return_param msg string 提示说明
 * @remark 支持多种筛选条件和排序，支持分页查询
 * @number 12
 */
func callGetUserList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC User GetUserList")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 创建 gRPC 客户端
	client := v5.NewUserClient(conn)
	req := &v5.GetUserListReq{
		GradeId:       getInt32FromMapWithDefault(reqData, "grade_id", 0),
		LevelId:       getInt32FromMapWithDefault(reqData, "level_id", 0),
		Status:        getInt32FromMapWithDefault(reqData, "status", 0),
		Username:      getStringFromMap(reqData, "username"),
		Realname:      getStringFromMap(reqData, "realname"),
		AgentUsername: getStringFromMap(reqData, "agent_username"),
		Mobile:        getStringFromMap(reqData, "mobile"),
		Page:          getInt32FromMapWithDefault(reqData, "page", 1),
		Size:          getInt32FromMapWithDefault(reqData, "size", 20),
		SortField:     getStringFromMap(reqData, "sort_field"),
		SortRule:      getInt32FromMapWithDefault(reqData, "sort_rule", 1),
		CardNo:        getStringFromMap(reqData, "card_no"),
		Domain:        getStringFromMap(reqData, "domain"),
		StartDate:     getStringFromMap(reqData, "start_date"),
		EndDate:       getStringFromMap(reqData, "end_date"),
		Charge:        getInt32FromMapWithDefault(reqData, "charge", 0),
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Page: %d, Size: %d, Username: %s",
		req.Page, req.Size, req.Username)

	// 调用 gRPC 服务
	res, err := client.GetUserList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取会员列表失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/会员/会员列表
 * @title 编辑会员信息
 * @description 编辑会员信息的接口
 * @method post
 * @url /api/backend/app/update-user
 * @param id 必选 int 用户ID
 * @param login_password 可选 string 登录密码
 * @param pay_password 可选 string 资金密码
 * @param grade_id 必选 int 等级ID
 * @param level_id 必选 int 层级ID
 * @param agent_id 必选 int 代理ID
 * @param realname 可选 string 真实姓名
 * @param mobile 可选 string 手机号
 * @param email 可选 string 邮箱
 * @param qq 可选 string QQ号
 * @param birthday 可选 string 生日
 * @param sex 必选 int 性别
 * @param focus_level 必选 int 关注级别
 * @param balance_status 必选 int 资金状态
 * @param status 必选 int 状态
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"更新用户成功"}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @return_param msg string 提示说明
 * @remark 用于编辑会员的基本信息和状态
 * @number 13
 */
func callUpdateUser(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC User UpdateUser")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 创建 gRPC 客户端
	client := v5.NewUserClient(conn)
	req := &v5.UpdateUserReq{
		Id:            getInt32FromMapWithDefault(reqData, "id", 0),
		LoginPassword: getStringFromMap(reqData, "login_password"),
		PayPassword:   getStringFromMap(reqData, "pay_password"),
		GradeId:       getInt32FromMapWithDefault(reqData, "grade_id", 0),
		LevelId:       getInt32FromMapWithDefault(reqData, "level_id", 0),
		AgentId:       getInt32FromMapWithDefault(reqData, "agent_id", 0),
		Realname:      getStringFromMap(reqData, "realname"),
		Mobile:        getStringFromMap(reqData, "mobile"),
		Email:         getStringFromMap(reqData, "email"),
		Qq:            getStringFromMap(reqData, "qq"),
		Birthday:      getStringFromMap(reqData, "birthday"),
		Sex:           getInt32FromMapWithDefault(reqData, "sex", 0),
		FocusLevel:    getInt32FromMapWithDefault(reqData, "focus_level", 1),
		BalanceStatus: getInt32FromMapWithDefault(reqData, "balance_status", 1),
		Status:        getInt32FromMapWithDefault(reqData, "status", 1),
		Remark:        getStringFromMap(reqData, "remark"),
	}

	if req.Id <= 0 {
		util.WriteBadRequest(r, "用户ID不能为空")
		return nil
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - ID: %d, GradeId: %d, LevelId: %d",
		req.Id, req.GradeId, req.LevelId)

	// 调用 gRPC 服务
	res, err := client.UpdateUser(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "编辑会员信息失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/会员/会员列表
 * @title 获取会员基本信息
 * @description 获取会员基本信息的接口
 * @method get
 * @url /api/backend/app/user-basic-info
 * @param id 必选 int 用户ID
 * @return {"code":0,"msg":"success","data":{"user":{"id":1,"username":"test","grade_name":"普通会员","level_name":"一级","balance":"100.00","register_time":"2023-01-01 12:00:00","agent_name":"agent1","realname":"张三","mobile":"138****1234","email":"test@example.com","sex":1,"birthday":"1990-01-01","qq":"123456789","balance_status":1,"focus_level":1,"remark":"","status":1,"device":"电脑","is_online":0,"banks":[{"bank_name":"工商银行","card_no":"6222****1234"}]}}}
 * @return_param code int 状态码
 * @return_param data object 主要数据
 * @return_param data.user object 用户基本信息
 * @return_param msg string 提示说明
 * @remark 获取指定用户的详细基本信息，包括银行卡信息
 * @number 14
 */
func callGetUserBasicInfo(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC User GetUserBasicInfo")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 创建 gRPC 客户端
	client := v5.NewUserClient(conn)
	req := &v5.GetUserBasicInfoReq{
		Id: getInt32FromMapWithDefault(reqData, "id", 0),
	}

	if req.Id <= 0 {
		util.WriteBadRequest(r, "用户ID不能为空")
		return nil
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - ID: %d", req.Id)

	// 调用 gRPC 服务
	res, err := client.GetUserBasicInfo(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		if strings.Contains(err.Error(), "不存在") {
			util.WriteBadRequest(r, "用户不存在")
		} else {
			util.WriteInternalError(r, "获取会员基本信息失败，请稍后重试")
		}
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/会员/会员等级
 * @title 获取会员等级列表
 * @description 获取会员等级列表的接口
 * @method get
 * @url /api/backend/app/user-grades
 * @param site_id 可选 int 站点ID (默认: 1)
 * @return {"code":200,"msg":"获取数据成功","data":[{"id":1,"name":"普通会员","points_upgrade":0,"bonus_upgrade":"0.00","bonus_birthday":"0.00","rebate_percent_sports":"0.00","rebate_percent_lottery":"0.00","rebate_percent_live":"0.00","rebate_percent_egame":"0.00","rebate_percent_poker":"0.00","user_count":10,"fields_disable":"","auto_providing":"","activities":[]}]}
 * @return_param code int 状态码
 * @return_param data array 等级列表
 * @return_param data.id int 等级ID
 * @return_param data.name string 等级名称
 * @return_param data.points_upgrade int 升级所需积分
 * @return_param data.bonus_upgrade string 升级赠送彩金
 * @return_param data.bonus_birthday string 生日彩金
 * @return_param data.rebate_percent_sports string 体育返水比例
 * @return_param data.rebate_percent_lottery string 彩票返水比例
 * @return_param data.rebate_percent_live string 真人视讯返水比例
 * @return_param data.rebate_percent_egame string 电子游戏返水比例
 * @return_param data.rebate_percent_poker string 扑克返水比例
 * @return_param data.user_count int 该等级用户数量
 * @return_param data.fields_disable string 禁用字段配置
 * @return_param data.auto_providing string 自动发放配置
 * @return_param data.activities array 关联活动列表
 * @return_param msg string 提示说明
 * @remark 获取站点的所有会员等级信息
 * @number 15
 */
func callGetUserGrades(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC User GetUserGrades")

	// 获取站点ID
	siteId := r.Get("site_id", 1).Int32()

	// 创建 gRPC 客户端
	client := v5.NewUserClient(conn)
	req := &v5.GetUserGradesReq{
		SiteId: siteId,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - SiteId: %d", req.SiteId)

	// 调用 gRPC 服务
	res, err := client.GetUserGrades(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取会员等级列表失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/会员/会员等级
 * @title 保存会员等级
 * @description 保存会员等级的接口
 * @method post
 * @url /api/backend/app/save-user-grades
 * @param site_id 必选 int 站点ID
 * @param data 必选 array 等级数据列表
 * @param fields_disable 可选 string 禁用字段配置
 * @param auto_providing 可选 string 自动发放配置
 * @return {"code":200,"msg":"保存成功","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @remark 批量保存会员等级信息，支持新增和更新
 * @number 16
 */
func callSaveUserGrades(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC User SaveUserGrades")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取站点ID
	siteId := getInt32FromMapWithDefault(reqData, "site_id", 1)

	// 获取等级数据
	dataInterface, ok := reqData["data"]
	if !ok {
		util.WriteBadRequest(r, "等级数据不能为空")
		return nil
	}

	// 解析等级数据
	var gradeInfos []*v5.UserGradeInfo
	if dataArray, ok := dataInterface.([]interface{}); ok {
		for _, item := range dataArray {
			if gradeMap, ok := item.(map[string]interface{}); ok {
				gradeInfo := &v5.UserGradeInfo{
					Id:                   getInt32FromMap(gradeMap, "id"),
					Name:                 getStringFromMap(gradeMap, "name"),
					PointsUpgrade:        getInt32FromMap(gradeMap, "points_upgrade"),
					BonusUpgrade:         getFloat64FromMap(gradeMap, "bonus_upgrade"),
					BonusBirthday:        getFloat64FromMap(gradeMap, "bonus_birthday"),
					RebatePercentSports:  getFloat64FromMap(gradeMap, "rebate_percent_sports"),
					RebatePercentLottery: getFloat64FromMap(gradeMap, "rebate_percent_lottery"),
					RebatePercentLive:    getFloat64FromMap(gradeMap, "rebate_percent_live"),
					RebatePercentEgame:   getFloat64FromMap(gradeMap, "rebate_percent_egame"),
					RebatePercentPoker:   getFloat64FromMap(gradeMap, "rebate_percent_poker"),
				}
				gradeInfos = append(gradeInfos, gradeInfo)
			}
		}
	} else {
		util.WriteBadRequest(r, "等级数据格式错误")
		return nil
	}

	if len(gradeInfos) == 0 {
		util.WriteBadRequest(r, "等级数据不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v5.NewUserClient(conn)
	req := &v5.SaveUserGradesReq{
		SiteId:        siteId,
		Data:          gradeInfos,
		FieldsDisable: getStringFromMap(reqData, "fields_disable"),
		AutoProviding: getStringFromMap(reqData, "auto_providing"),
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - SiteId: %d, GradeCount: %d", req.SiteId, len(req.Data))

	// 调用 gRPC 服务
	res, err := client.SaveUserGrades(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "保存会员等级失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/会员/会员等级
 * @title 删除会员等级
 * @description 删除会员等级的接口
 * @method post
 * @url /api/backend/app/delete-user-grades
 * @param site_id 必选 int 站点ID
 * @param id 必选 int 等级ID
 * @return {"code":200,"msg":"删除成功","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @remark 删除指定的会员等级，会检查是否为默认等级和是否有用户使用
 * @number 17
 */
func callDeleteUserGrades(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC User DeleteUserGrades")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	siteId := getInt32FromMapWithDefault(reqData, "site_id", 1)
	id := getInt32FromMap(reqData, "id")

	if id <= 0 {
		util.WriteBadRequest(r, "等级ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v5.NewUserClient(conn)
	req := &v5.DeleteUserGradesReq{
		SiteId: siteId,
		Id:     id,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - SiteId: %d, Id: %d", req.SiteId, req.Id)

	// 调用 gRPC 服务
	res, err := client.DeleteUserGrades(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		if strings.Contains(err.Error(), "默认等级") {
			util.WriteBadRequest(r, "默认等级不能删除")
		} else if strings.Contains(err.Error(), "有会员") {
			util.WriteBadRequest(r, "该等级下面有会员，请先将会员变更为其它等级，才能删除该等级")
		} else {
			util.WriteInternalError(r, "删除会员等级失败，请稍后重试")
		}
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/会员/会员日志
 * @title 获取会员登录日志
 * @description 获取会员登录日志列表
 * @method get
 * @url /api/backend/app/user-login-logs
 * @param username 可选 string 用户名
 * @param ip 可选 string 登录IP
 * @param start_time 可选 string 开始时间
 * @param end_time 可选 string 结束时间
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"user_id":123,"username":"testuser","referer_url":"","login_url":"","login_time":"2024-01-01 10:00:00","login_ip":"192.168.1.1","login_address":"北京市","os":"Windows","network":"","screen":"1920x1080","browser":"Chrome","device":"电脑","is_robot":0,"created_at":"2024-01-01 10:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 登录日志列表
 * @return_param data.count int 总数量
 * @remark 获取会员登录日志，支持按用户名、IP、时间范围筛选
 * @number 18
 */
func callGetUserLoginLogs(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC User GetUserLoginLogs")

	// 获取查询参数
	username := r.Get("username", "").String()
	ip := r.Get("ip", "").String()
	startTime := r.Get("start_time", "").String()
	endTime := r.Get("end_time", "").String()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v5.NewUserClient(conn)
	req := &v5.GetUserLoginLogsReq{
		Username:  username,
		Ip:        ip,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		Size:      size,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Username: %s, IP: %s, Page: %d, Size: %d", req.Username, req.Ip, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetUserLoginLogs(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取会员登录日志失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
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

/**
 * showdoc
 * @catalog 后台/财务/账变记录
 * @title 获取账变记录
 * @description 获取会员账变记录列表
 * @method get
 * @url /api/backend/app/balance-changes
 * @param username 可选 string 用户名
 * @param change_type 可选 int 账变类型 1=入款 2=出款
 * @param trade_type 可选 int 交易类型
 * @param start_time 可选 string 开始时间
 * @param end_time 可选 string 结束时间
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"trade_type":2,"trade_type_name":"充值","user_id":123,"username":"testuser","trade_no":"T123456","balance_old":100.0,"money":50.0,"balance_new":150.0,"balance_frozen":0.0,"status":2,"status_name":"成功","remark":"","created_at":"2024-01-01 10:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 账变记录列表
 * @return_param data.count int 总数量
 * @remark 获取会员账变记录，支持按用户名、账变类型、交易类型、时间范围筛选
 * @number 1
 */
func callGetBalanceChanges(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetBalanceChanges")

	// 获取查询参数
	username := r.Get("username", "").String()
	changeType := r.Get("change_type", 0).Int32()
	tradeType := r.Get("trade_type", 0).Int32()
	startTime := r.Get("start_time", "").String()
	endTime := r.Get("end_time", "").String()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetBalanceChangesReq{
		Username:   username,
		ChangeType: changeType,
		TradeType:  tradeType,
		StartTime:  startTime,
		EndTime:    endTime,
		Page:       page,
		Size:       size,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Username: %s, ChangeType: %d, Page: %d, Size: %d", req.Username, req.ChangeType, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetBalanceChanges(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取账变记录失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/充值记录
 * @title 获取充值记录
 * @description 获取在线支付充值记录列表
 * @method get
 * @url /api/backend/app/recharge-payment
 * @param username 可选 string 用户名
 * @param gateway 可选 int 网关类型
 * @param payment_id 可选 int 支付ID
 * @param account_id 可选 int 账号ID
 * @param status 可选 int 状态
 * @param trade_no 可选 string 流水号
 * @param domain 可选 string 域名
 * @param start_time 可选 string 开始时间
 * @param end_time 可选 string 结束时间
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"user_id":123,"username":"testuser","gateway":1,"gateway_name":"在线支付","payment_id":1,"trade_no":"P123456","money":100.0,"fee":0.0,"status":2,"status_name":"成功","created_at":"2024-01-01 10:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 充值记录列表
 * @return_param data.count int 总数量
 * @remark 获取在线支付充值记录，支持多条件筛选
 * @number 2
 */
func callGetRechargePayments(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetRechargePayments")

	// 获取查询参数
	username := r.Get("username", "").String()
	gateway := r.Get("gateway", 0).Int32()
	paymentId := r.Get("payment_id", 0).Int32()
	accountId := r.Get("account_id", 0).Int32()
	status := r.Get("status", 0).Int32()
	tradeNo := r.Get("trade_no", "").String()
	domain := r.Get("domain", "").String()
	startTime := r.Get("start_time", "").String()
	endTime := r.Get("end_time", "").String()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetRechargePaymentsReq{
		Username:  username,
		Gateway:   gateway,
		PaymentId: paymentId,
		AccountId: accountId,
		Status:    status,
		TradeNo:   tradeNo,
		Domain:    domain,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		Size:      size,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Username: %s, Gateway: %d, Page: %d, Size: %d", req.Username, req.Gateway, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetRechargePayments(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取充值记录失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/充值记录
 * @title 获取后台加款记录
 * @description 获取后台手动加款记录列表
 * @method get
 * @url /api/backend/app/recharge-manual
 * @param username 可选 string 用户名
 * @param status 可选 int 状态
 * @param start_time 可选 string 开始时间
 * @param end_time 可选 string 结束时间
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"user_id":123,"username":"testuser","trade_no":"M123456","money":100.0,"status":2,"status_name":"成功","admin_id":1,"admin_name":"admin","remark":"手动加款","created_at":"2024-01-01 10:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 后台加款记录列表
 * @return_param data.count int 总数量
 * @remark 获取后台手动加款记录，支持按用户名、状态、时间范围筛选
 * @number 3
 */
func callGetRechargeManuals(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetRechargeManuals")

	// 获取查询参数
	username := r.Get("username", "").String()
	status := r.Get("status", 0).Int32()
	startTime := r.Get("start_time", "").String()
	endTime := r.Get("end_time", "").String()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetRechargeManualsReq{
		Username:  username,
		Status:    status,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		Size:      size,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Username: %s, Status: %d, Page: %d, Size: %d", req.Username, req.Status, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetRechargeManuals(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取后台加款记录失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/充值记录
 * @title 确认支付订单
 * @description 手动确认在线支付订单
 * @method post
 * @url /api/backend/app/confirm-payment-order
 * @param id 必选 int 订单ID
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"订单确认成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @remark 手动确认在线支付订单，更新订单状态并处理余额
 * @number 4
 */
func callConfirmPaymentOrder(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance ConfirmPaymentOrder")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt64FromMap(reqData, "id")
	remark := getStringFromMap(reqData, "remark")

	if id <= 0 {
		util.WriteBadRequest(r, "订单ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.ConfirmPaymentOrderReq{
		Id:     id,
		Remark: remark,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Id: %d, Remark: %s", req.Id, req.Remark)

	// 调用 gRPC 服务
	res, err := client.ConfirmPaymentOrder(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "确认支付订单失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/提现记录
 * @title 获取提现记录
 * @description 获取会员提现记录列表
 * @method get
 * @url /api/backend/app/withdraws
 * @param username 可选 string 用户名
 * @param status 可选 int 状态
 * @param trade_no 可选 string 流水号
 * @param domain 可选 string 域名
 * @param start_time 可选 string 开始时间
 * @param end_time 可选 string 结束时间
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"user_id":123,"username":"testuser","trade_no":"W123456","money":100.0,"fee":5.0,"bank_name":"工商银行","card_account":"张三","card_no":"6222****1234","status":1,"status_name":"待审核","created_at":"2024-01-01 10:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 提现记录列表
 * @return_param data.count int 总数量
 * @remark 获取会员提现记录，支持多条件筛选
 * @number 5
 */
func callGetWithdraws(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetWithdraws")

	// 获取查询参数
	username := r.Get("username", "").String()
	status := r.Get("status", 0).Int32()
	tradeNo := r.Get("trade_no", "").String()
	domain := r.Get("domain", "").String()
	startTime := r.Get("start_time", "").String()
	endTime := r.Get("end_time", "").String()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetWithdrawsReq{
		Username:  username,
		Status:    status,
		TradeNo:   tradeNo,
		Domain:    domain,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		Size:      size,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Username: %s, Status: %d, Page: %d, Size: %d", req.Username, req.Status, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetWithdraws(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取提现记录失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/提现记录
 * @title 获取后台提现记录
 * @description 获取后台手动提现记录列表
 * @method get
 * @url /api/backend/app/withdraw-manual
 * @param username 可选 string 用户名
 * @param status 可选 int 状态
 * @param start_time 可选 string 开始时间
 * @param end_time 可选 string 结束时间
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"user_id":123,"username":"testuser","trade_no":"WM123456","money":100.0,"status":2,"status_name":"成功","admin_id":1,"admin_name":"admin","remark":"手动提现","created_at":"2024-01-01 10:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 后台提现记录列表
 * @return_param data.count int 总数量
 * @remark 获取后台手动提现记录，支持按用户名、状态、时间范围筛选
 * @number 6
 */
func callGetWithdrawManuals(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetWithdrawManuals")

	// 获取查询参数
	username := r.Get("username", "").String()
	status := r.Get("status", 0).Int32()
	startTime := r.Get("start_time", "").String()
	endTime := r.Get("end_time", "").String()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetWithdrawManualsReq{
		Username:  username,
		Status:    status,
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		Size:      size,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Username: %s, Status: %d, Page: %d, Size: %d", req.Username, req.Status, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetWithdrawManuals(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取后台提现记录失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/提现记录
 * @title 获取提现审核信息
 * @description 获取提现记录的详细审核信息
 * @method get
 * @url /api/backend/app/withdraw-review
 * @param id 必选 int 提现记录ID
 * @return {"code":0,"msg":"success","data":{"data":{"id":1,"user_id":123,"username":"testuser","trade_no":"W123456","money":100.0,"fee":5.0,"bank_name":"工商银行","card_account":"张三","card_no":"6222021234567890","status":1,"remark":"","created_at":"2024-01-01 10:00:00","user_realname":"张三","user_mobile":"13800138000","user_balance":500.0,"user_balance_frozen":100.0,"total_recharge":1000.0,"total_withdraw":200.0,"withdraw_count":3}}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.data object 审核信息
 * @remark 获取提现记录的详细审核信息，包含用户信息和统计数据
 * @number 7
 */
func callGetWithdrawReview(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetWithdrawReview")

	// 获取查询参数
	id := r.Get("id", 0).Int64()

	if id <= 0 {
		util.WriteBadRequest(r, "提现记录ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetWithdrawReviewReq{
		Id: id,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Id: %d", req.Id)

	// 调用 gRPC 服务
	res, err := client.GetWithdrawReview(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取提现审核信息失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/提现记录
 * @title 处理提现
 * @description 处理提现申请（确认/拒绝/补单）
 * @method post
 * @url /api/backend/app/deal-with-withdraw
 * @param id 必选 int 提现记录ID
 * @param type 必选 int 处理类型 1=确认 0=拒绝 2=补单
 * @param fee 可选 float 手续费
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"处理成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @remark 处理提现申请，支持确认、拒绝、补单操作
 * @number 8
 */
func callDealWithWithdraw(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance DealWithWithdraw")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt64FromMap(reqData, "id")
	dealType := getInt32FromMap(reqData, "type")
	fee := getFloat64FromMap(reqData, "fee")
	remark := getStringFromMap(reqData, "remark")

	if id <= 0 {
		util.WriteBadRequest(r, "提现记录ID不能为空")
		return nil
	}

	if dealType < 0 || dealType > 2 {
		util.WriteBadRequest(r, "处理类型无效")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.DealWithWithdrawReq{
		Id:     id,
		Type:   dealType,
		Fee:    fee,
		Remark: remark,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Id: %d, Type: %d, Fee: %f, Remark: %s", req.Id, req.Type, req.Fee, req.Remark)

	// 调用 gRPC 服务
	res, err := client.DealWithWithdraw(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "处理提现失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
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

/**
 * showdoc
 * @catalog 后台/财务/加扣款
 * @title 查询用户余额
 * @description 查询指定用户的账户余额信息
 * @method post
 * @url /api/backend/balance/query-user-balance
 * @param username 必选 string 用户名
 * @return {"code":0,"msg":"success","data":{"data":{"user_id":123,"username":"testuser","balance":1000.0,"balance_frozen":0.0,"points":100.0,"last_update_time":"2024-01-01 10:00:00"}}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.data object 用户余额信息
 * @remark 查询指定用户的账户余额、冻结余额和积分信息
 * @number 9
 */
func callQueryUserBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance QueryUserBalance")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数 - 支持数字和字符串类型
	userId := getInt32FromMap(reqData, "user_id")

	if userId <= 0 {
		util.WriteBadRequest(r, "用户ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.QueryUserBalanceReq{
		UserId: userId,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - user_id: %d", req.UserId)

	// 调用 gRPC 服务
	res, err := client.QueryUserBalance(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "查询用户余额失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/加扣款
 * @title 查询游戏余额
 * @description 查询指定用户在第三方游戏的余额信息
 * @method post
 * @url /api/backend/balance/query-game-balance
 * @param game_id 必选 int 游戏ID
 * @param user_id 必选 int 用户ID
 * @return {"code":0,"msg":"success","data":{"data":{"game_id":1,"game_name":"测试游戏","user_id":123,"username":"testuser","balance":1000.0,"last_update_time":"2024-01-01 10:00:00"}}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.data object 游戏余额信息
 * @remark 查询指定用户在第三方游戏平台的余额信息
 * @number 10
 */
func callQueryGameBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance QueryGameBalance")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	gameId := getInt32FromMap(reqData, "game_id")
	userId := getInt32FromMap(reqData, "user_id")

	if gameId <= 0 {
		util.WriteBadRequest(r, "游戏ID不能为空")
		return nil
	}
	if userId <= 0 {
		util.WriteBadRequest(r, "用户ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.QueryGameBalanceReq{
		GameId: gameId,
		UserId: userId,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - GameId: %d, UserId: %d", req.GameId, req.UserId)

	// 调用 gRPC 服务
	res, err := client.QueryGameBalance(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "查询游戏余额失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/加扣款
 * @title 手动操作用户余额
 * @description 手动给用户加款或减款
 * @method post
 * @url /api/backend/balance/manual-user-balance
 * @param user_id 必选 int 用户ID
 * @param type 必选 int 操作类型 1=加款 2=扣款
 * @param money 必选 float 操作金额
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"操作成功","balance_old":1000.0,"balance_new":1100.0}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @return_param data.balance_old float 操作前余额
 * @return_param data.balance_new float 操作后余额
 * @remark 手动给用户加款或扣款，会记录账变和操作日志
 * @number 11
 */
func callManualUserBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance ManualUserBalance")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	userId := getInt32FromMap(reqData, "user_id")
	operationType := getInt32FromMap(reqData, "type")
	money := getFloat64FromMap(reqData, "money")
	remark := getStringFromMap(reqData, "remark")

	if userId <= 0 {
		util.WriteBadRequest(r, "用户ID不能为空")
		return nil
	}

	if money <= 0 {
		util.WriteBadRequest(r, "操作金额必须大于0")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.ManualUserBalanceReq{
		UserId: userId,
		Type:   operationType,
		Money:  money,
		Remark: remark,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - UserId: %d, Type: %d, Money: %f, Remark: %s", req.UserId, req.Type, req.Money, req.Remark)

	// 调用 gRPC 服务
	res, err := client.ManualUserBalance(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "手动操作用户余额失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/支付接口
 * @title 获取支付接口列表
 * @description 获取支付接口账户列表
 * @method get
 * @url /api/backend/balance/payment-accounts
 * @param payment_id 可选 int 支付ID
 * @param status 可选 int 状态
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"payment_id":1,"payment_name":"测试支付","account_name":"测试账户","merchant_id":"123456","merchant_key":"abcd****efgh","gateway_url":"https://api.test.com","min_money":10.0,"max_money":50000.0,"status":1,"status_name":"启用","sort":1,"remark":"测试","created_at":"2024-01-01 10:00:00"}],"count":1}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 支付接口列表
 * @return_param data.count int 总数量
 * @remark 获取支付接口账户列表，支持按支付ID、状态筛选
 * @number 12
 */
func callGetPaymentAccounts(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetPaymentAccounts")

	// 获取查询参数
	paymentId := r.Get("payment_id", 0).Int32()
	status := r.Get("status", 0).Int32()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetPaymentAccountsReq{
		PaymentId: paymentId,
		Status:    status,
		Page:      page,
		Size:      size,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - PaymentId: %d, Status: %d, Page: %d, Size: %d", req.PaymentId, req.Status, req.Page, req.Size)

	// 调用 gRPC 服务
	res, err := client.GetPaymentAccounts(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取支付接口列表失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/支付接口
 * @title 创建支付接口
 * @description 添加新的支付接口账户
 * @method post
 * @url /api/backend/balance/create-payment-account
 * @param payment_id 必选 int 支付ID
 * @param account_name 必选 string 账户名称
 * @param merchant_id 必选 string 商户ID
 * @param merchant_key 必选 string 商户密钥
 * @param gateway_url 可选 string 网关地址
 * @param notify_url 可选 string 通知地址
 * @param return_url 可选 string 返回地址
 * @param min_money 可选 float 最小金额
 * @param max_money 可选 float 最大金额
 * @param status 可选 int 状态 0=禁用 1=启用
 * @param sort 可选 int 排序
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"创建成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @remark 添加新的支付接口账户配置
 * @number 13
 */
func callCreatePaymentAccount(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance CreatePaymentAccount")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	paymentId := getInt32FromMap(reqData, "payment_id")
	gateway := getInt32FromMap(reqData, "gateway")
	name := getStringFromMap(reqData, "name")
	domain := getStringFromMap(reqData, "domain")
	merchantNo := getStringFromMap(reqData, "merchant_no")
	md5Key := getStringFromMap(reqData, "md5_key")
	eachMin := getFloat64FromMap(reqData, "each_min")
	eachMax := getFloat64FromMap(reqData, "each_max")
	dailyMax := getFloat64FromMap(reqData, "daily_max")
	status := getInt32FromMapWithDefault(reqData, "status", 1)
	sort := getInt32FromMap(reqData, "sort")
	publicKey := getStringFromMap(reqData, "public_key")
	privateKey := getStringFromMap(reqData, "private_key")
	isDecimal := getInt32FromMap(reqData, "is_decimal")
	isInt := getInt32FromMap(reqData, "is_int")
	moneyList := getStringFromMap(reqData, "money_list")

	if paymentId <= 0 {
		util.WriteBadRequest(r, "支付ID不能为空")
		return nil
	}
	if name == "" {
		util.WriteBadRequest(r, "接口名称不能为空")
		return nil
	}
	if merchantNo == "" {
		util.WriteBadRequest(r, "商户号不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.CreatePaymentAccountReq{
		PaymentId:  paymentId,
		Gateway:    gateway,
		Name:       name,
		Domain:     domain,
		MerchantNo: merchantNo,
		Md5Key:     md5Key,
		EachMin:    eachMin,
		EachMax:    eachMax,
		DailyMax:   dailyMax,
		Status:     status,
		Sort:       sort,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		IsDecimal:  isDecimal,
		IsInt:      isInt,
		MoneyList:  moneyList,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - PaymentId: %d, Name: %s", req.PaymentId, req.Name)

	// 调用 gRPC 服务
	res, err := client.CreatePaymentAccount(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "创建支付接口失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/支付接口
 * @title 获取支付接口编辑信息
 * @description 获取支付接口的详细信息用于编辑
 * @method get
 * @url /api/backend/balance/update-payment-account
 * @param id 必选 int 支付接口ID
 * @return {"code":0,"msg":"success","data":{"data":{"id":1,"payment_id":1,"payment_name":"测试支付","account_name":"测试账户","merchant_id":"123456","merchant_key":"abcdefgh","gateway_url":"https://api.test.com","min_money":10.0,"max_money":50000.0,"status":1,"status_name":"启用","sort":1,"remark":"测试","created_at":"2024-01-01 10:00:00"}}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.data object 支付接口详细信息
 * @remark 获取支付接口的完整信息，包括敏感数据（用于编辑）
 * @number 14
 */
func callGetPaymentAccountUpdate(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetPaymentAccountUpdate")

	// 获取查询参数
	id := r.Get("id", 0).Int32()

	if id <= 0 {
		util.WriteBadRequest(r, "支付接口ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetPaymentAccountUpdateReq{
		Id: id,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Id: %d", req.Id)

	// 调用 gRPC 服务
	res, err := client.GetPaymentAccountUpdate(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取支付接口信息失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/支付接口
 * @title 更新支付接口
 * @description 更新支付接口账户信息
 * @method post
 * @url /api/backend/balance/update-payment-account
 * @param id 必选 int 支付接口ID
 * @param payment_id 必选 int 支付ID
 * @param account_name 必选 string 账户名称
 * @param merchant_id 必选 string 商户ID
 * @param merchant_key 必选 string 商户密钥
 * @param gateway_url 可选 string 网关地址
 * @param notify_url 可选 string 通知地址
 * @param return_url 可选 string 返回地址
 * @param min_money 可选 float 最小金额
 * @param max_money 可选 float 最大金额
 * @param status 可选 int 状态 0=禁用 1=启用
 * @param sort 可选 int 排序
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"更新成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @remark 更新支付接口账户配置信息
 * @number 15
 */
func callUpdatePaymentAccount(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance UpdatePaymentAccount")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt32FromMap(reqData, "id")
	paymentId := getInt32FromMap(reqData, "payment_id")
	gateway := getInt32FromMap(reqData, "gateway")
	name := getStringFromMap(reqData, "name")
	domain := getStringFromMap(reqData, "domain")
	merchantNo := getStringFromMap(reqData, "merchant_no")
	md5Key := getStringFromMap(reqData, "md5_key")
	eachMin := getFloat64FromMap(reqData, "each_min")
	eachMax := getFloat64FromMap(reqData, "each_max")
	dailyMax := getFloat64FromMap(reqData, "daily_max")
	status := getInt32FromMapWithDefault(reqData, "status", 1)
	sort := getInt32FromMap(reqData, "sort")
	publicKey := getStringFromMap(reqData, "public_key")
	privateKey := getStringFromMap(reqData, "private_key")
	isDecimal := getInt32FromMap(reqData, "is_decimal")
	isInt := getInt32FromMap(reqData, "is_int")
	moneyList := getStringFromMap(reqData, "money_list")

	if id <= 0 {
		util.WriteBadRequest(r, "支付接口ID不能为空")
		return nil
	}
	if paymentId <= 0 {
		util.WriteBadRequest(r, "支付ID不能为空")
		return nil
	}
	if name == "" {
		util.WriteBadRequest(r, "接口名称不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.UpdatePaymentAccountReq{
		Id:         id,
		PaymentId:  paymentId,
		Gateway:    gateway,
		Name:       name,
		Domain:     domain,
		MerchantNo: merchantNo,
		Md5Key:     md5Key,
		EachMin:    eachMin,
		EachMax:    eachMax,
		DailyMax:   dailyMax,
		Status:     status,
		Sort:       sort,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		IsDecimal:  isDecimal,
		IsInt:      isInt,
		MoneyList:  moneyList,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Id: %d, PaymentId: %d, Name: %s", req.Id, req.PaymentId, req.Name)

	// 调用 gRPC 服务
	res, err := client.UpdatePaymentAccount(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "更新支付接口失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/支付接口
 * @title 删除支付接口
 * @description 删除支付接口账户
 * @method post
 * @url /api/backend/balance/delete-payment-account
 * @param id 必选 int 支付接口ID
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"删除成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @remark 删除支付接口账户，会检查是否有关联的支付记录
 * @number 16
 */
func callDeletePaymentAccount(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance DeletePaymentAccount")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt32FromMap(reqData, "id")

	if id <= 0 {
		util.WriteBadRequest(r, "支付接口ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.DeletePaymentAccountReq{
		Id: id,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Id: %d", req.Id)

	// 调用 gRPC 服务
	res, err := client.DeletePaymentAccount(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "删除支付接口失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/公共
 * @title 获取操作类型列表
 * @description 获取手动操作的操作类型列表
 * @method GET
 * @url /api/backend/balance/options-manual
 * @return_param data object 操作类型列表
 * @return_param data.list map 操作类型映射 {typeCode: typeName}
 * @remark 返回所有可用的操作类型
 * @number 17
 */
func callGetManualList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetManualList")

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetManualListReq{}

	// 调用 gRPC 服务
	res, err := client.GetManualList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取操作类型列表失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
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

/**
 * showdoc
 * @catalog 后台/游戏/游戏管理
 * @title 获取游戏列表
 * @description 获取游戏列表的接口
 * @method get
 * @url /api/backend/game/games
 * @param seachBlance 可选 bool 是否查询余额
 * @return {"code":0,"msg":"success","data":{"count":4,"data":{"4":{"games":[{"code":"GFG_GAME","db_name":"gfg_game","id":15,"image_code":"nx46","name":"GFG电游","platform":"GFG","sort":1,"status":1,"type":4},{"code":"IG_JILI_GAME","db_name":"ig_jili_game","id":16,"image_code":"nx46","name":"IG_JILI电游","platform":"IG_JILI","sort":4,"status":1,"type":4},{"code":"ONE_GAME","db_name":"one_api_game","id":17,"image_code":"nx46","name":"ONE电游","platform":"ONE","sort":4,"status":1,"type":4},{"code":"IG_PP_GAME","db_name":"ig_pp_game","id":18,"image_code":"nx46","name":"IG_PP电游","platform":"IG_PP","sort":4,"status":1,"type":4}]}}}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func callGetGames(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGames")

	// 获取站点ID（从用户信息中获取）
	siteId := int32(1) // 暂时写死，后续从用户信息中获取

	// 获取查询参数
	searchBalance := r.Get("seachBlance").Bool()

	// 构建请求
	req := &v7.GetGamesReq{
		SiteId:        siteId,
		SearchBalance: searchBalance,
	}

	// 调用gRPC服务
	client := v7.NewGameClient(conn)
	res, err := client.GetGames(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGames failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏管理
 * @title 修改站点游戏开关
 * @description 修改站点游戏开关的接口
 * @method post
 * @url /api/backend/game/update-games
 * @param data 必选 array 游戏数据数组
 * @return {"code":0,"msg":"success","data":{"message":"设置游戏开关成功"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 2
 */
func callUpdateGames(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game UpdateGames")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.LogWithTrace(ctx, "error", "failed to get request data: %v", err)
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	// 获取站点ID（从用户信息中获取）
	siteId := int32(1) // 暂时写死，后续从用户信息中获取

	// 解析游戏数据
	dataInterface, exists := reqData["data"]
	if !exists {
		util.WriteBadRequest(r, "缺少data参数")
		return fmt.Errorf("missing data parameter")
	}

	// 转换为游戏更新信息数组
	var gameUpdates []*v7.GameUpdateInfo

	switch dataValue := dataInterface.(type) {
	case []interface{}:
		for _, item := range dataValue {
			if gameMap, ok := item.(map[string]interface{}); ok {
				gameUpdate := &v7.GameUpdateInfo{}

				if id, exists := gameMap["id"]; exists {
					if idFloat, ok := id.(float64); ok {
						gameUpdate.Id = int32(idFloat)
					}
				}

				if status, exists := gameMap["status"]; exists {
					if statusFloat, ok := status.(float64); ok {
						gameUpdate.Status = int32(statusFloat)
					}
				}

				if sort, exists := gameMap["sort"]; exists {
					if sortFloat, ok := sort.(float64); ok {
						gameUpdate.Sort = int32(sortFloat)
					}
				}

				gameUpdates = append(gameUpdates, gameUpdate)
			}
		}
	default:
		util.WriteBadRequest(r, "data参数格式错误")
		return fmt.Errorf("invalid data parameter format")
	}

	// 构建请求
	req := &v7.UpdateGamesReq{
		SiteId: siteId,
		Data:   gameUpdates,
	}

	// 调用gRPC服务
	client := v7.NewGameClient(conn)
	res, err := client.UpdateGames(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game UpdateGames failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏记录
 * @title 获取游戏记录
 * @description 获取游戏记录的接口
 * @method post
 * @url /api/backend/game/game-record
 * @param platform 必选 string 游戏平台代码 egame/jili/pg
 * @param start_date 可选 string 开始时间 格式: Y-m-d H:i:s
 * @param end_date 可选 string 结束时间 格式: Y-m-d H:i:s
 * @param page 可选 int 页码 默认1
 * @param size 可选 int 每页数量 默认20
 * @param username 可选 string 用户名
 * @param type 可选 int 记录类型 1=投注记录 2=中奖记录
 * @param keywords 可选 string 关键词搜索
 * @return {"code":0,"msg":"success","data":{"data":[{"id":1,"site_id":1,"user_id":123,"username":"test","platform_code":"egame","game_code":"GAME001","game_name":"测试游戏","game_type":"电子游戏","order_id":"ORDER123","round_id":"ROUND123","bet_amount":"100.000000","win_amount":"200.000000","currency":"CNY","bet_time":"2024-01-01 12:00:00","settle_time":"2024-01-01 12:01:00","status":1,"raw_data":"{}","remark":"","created_at":"2024-01-01 12:00:00","updated_at":"2024-01-01 12:01:00"}],"total":1,"current_page":1,"per_page":20,"last_page":1,"total_amount":"100.00","all_game_total_amount":"120.00"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 3
 */
func callGetGameRecords(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGameRecords")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.LogWithTrace(ctx, "error", "failed to get request data: %v", err)
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	// 获取站点ID（从用户信息中获取）
	siteId := int32(1) // 暂时写死，后续从用户信息中获取

	// 解析请求参数
	platform := ""
	if p, exists := reqData["platform"]; exists {
		if pStr, ok := p.(string); ok {
			platform = pStr
		}
	}
	if platform == "" {
		util.WriteBadRequest(r, "缺少platform参数")
		return fmt.Errorf("missing platform parameter")
	}

	startDate := ""
	if sd, exists := reqData["start_date"]; exists {
		if sdStr, ok := sd.(string); ok {
			startDate = sdStr
		}
	}

	endDate := ""
	if ed, exists := reqData["end_date"]; exists {
		if edStr, ok := ed.(string); ok {
			endDate = edStr
		}
	}

	page := int32(1)
	if p, exists := reqData["page"]; exists {
		if pFloat, ok := p.(float64); ok {
			page = int32(pFloat)
		}
	}

	size := int32(20)
	if s, exists := reqData["size"]; exists {
		if sFloat, ok := s.(float64); ok {
			size = int32(sFloat)
		}
	}

	username := ""
	if u, exists := reqData["username"]; exists {
		if uStr, ok := u.(string); ok {
			username = uStr
		}
	}

	recordType := int32(0)
	if t, exists := reqData["type"]; exists {
		if tFloat, ok := t.(float64); ok {
			recordType = int32(tFloat)
		}
	}

	keywords := ""
	if k, exists := reqData["keywords"]; exists {
		if kStr, ok := k.(string); ok {
			keywords = kStr
		}
	}

	// 构建请求
	req := &v7.GetGameRecordsReq{
		SiteId:    siteId,
		Platform:  platform,
		StartDate: startDate,
		EndDate:   endDate,
		Page:      page,
		Size:      size,
		Username:  username,
		Type:      recordType,
		Keywords:  keywords,
	}

	// 调用gRPC服务
	client := v7.NewGameClient(conn)
	res, err := client.GetGameRecords(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGameRecords failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/消息管理
 * @title 获取消息列表
 * @description 获取消息列表的接口
 * @method get
 * @url /api/backend/app/messages
 * @param page 可选 int 页码
 * @param size 可选 int 每页数量
 * @return {"code":0,"msg":"success","data":{"message_list":[],"grade_list":[],"level_list":[],"status_list":[],"total":0,"current_page":1,"per_page":50}}
 * @number 11
 */
func callGetMessageList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Message GetMessageList")

	// 解析请求参数
	page := gconv.Int32(r.GetQuery("page", "1"))
	size := gconv.Int32(r.GetQuery("size", "50"))

	req := &v8.GetMessageListReq{
		Page: page,
		Size: size,
	}

	// 调用gRPC服务
	client := v8.NewMessageClient(conn)
	res, err := client.GetMessageList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Message GetMessageList failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/消息管理
 * @title 创建消息
 * @description 创建消息的接口
 * @method post
 * @url /api/backend/app/create-message
 * @param title 必选 string 消息标题
 * @param content 必选 string 消息内容
 * @param type 必选 int 消息类型 1:指定用户 2:等级 3:层级 4:状态
 * @param receiver 可选 string 接收者
 * @param grade_id 可选 int 等级ID
 * @param level_id 可选 int 层级ID
 * @param status 可选 int 状态
 * @param position 可选 int 弹框位置
 * @param alert_times 可选 int 弹框次数
 * @return {"code":0,"msg":"success","data":{}}
 * @number 12
 */
func callCreateMessage(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Message CreateMessage")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	req := &v8.CreateMessageReq{
		Title:      gconv.String(requestData["title"]),
		Content:    gconv.String(requestData["content"]),
		Type:       gconv.Int32(requestData["type"]),
		Receiver:   gconv.String(requestData["receiver"]),
		GradeId:    gconv.Int32(requestData["grade_id"]),
		LevelId:    gconv.Int32(requestData["level_id"]),
		Status:     gconv.Int32(requestData["status"]),
		Position:   gconv.Int32(requestData["position"]),
		AlertTimes: gconv.Int32(requestData["alert_times"]),
	}

	// 调用gRPC服务
	client := v8.NewMessageClient(conn)
	res, err := client.CreateMessage(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Message CreateMessage failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台
 * @title 获取用户消息
 * @description 获取用户消息的接口
 * @method get
 * @url /api/frontend/app/user-messages
 * @param user_id 必选 int 用户ID
 * @param page 可选 int 页码
 * @param size 可选 int 每页数量
 * @return {"code":0,"msg":"success","data":{"list":[],"total":0,"current_page":1,"per_page":20,"unread_message_count":0}}
 * @number 13
 */
func callGetUserMessages(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Message GetUserMessages")

	// 解析请求参数
	userId := gconv.Int32(r.GetQuery("user_id"))
	page := gconv.Int32(r.GetQuery("page", "1"))
	size := gconv.Int32(r.GetQuery("size", "20"))

	req := &v8.GetUserMessagesReq{
		UserId: userId,
		Page:   page,
		Size:   size,
	}

	// 调用gRPC服务
	client := v8.NewMessageClient(conn)
	res, err := client.GetUserMessages(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Message GetUserMessages failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台
 * @title 获取未读数量
 * @description 获取未读消息数量的接口
 * @method get
 * @url /api/frontend/app/unread-count
 * @param user_id 必选 int 用户ID
 * @return {"code":0,"msg":"success","data":{"count":0}}
 * @number 14
 */
func callGetUnreadCount(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Message GetUnreadCount")

	// 解析请求参数
	userId := gconv.Int32(r.GetQuery("user_id"))

	req := &v8.GetUnreadCountReq{
		UserId: userId,
	}

	// 调用gRPC服务
	client := v8.NewMessageClient(conn)
	res, err := client.GetUnreadCount(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Message GetUnreadCount failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台
 * @title 读取消息
 * @description 标记消息为已读的接口
 * @method post
 * @url /api/frontend/app/read-message
 * @param user_id 必选 int 用户ID
 * @param message_id 必选 int 消息ID
 * @return {"code":0,"msg":"success","data":{}}
 * @number 15
 */
func callReadMessage(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Message ReadMessage")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	req := &v8.ReadMessageReq{
		UserId:    gconv.Int32(requestData["user_id"]),
		MessageId: gconv.Int32(requestData["message_id"]),
	}

	// 调用gRPC服务
	client := v8.NewMessageClient(conn)
	res, err := client.ReadMessage(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Message ReadMessage failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/广告管理
 * @title 获取广告列表
 * @description 获取广告列表的接口
 * @method get
 * @url /api/backend/app/ads
 * @param page 可选 int 页码
 * @param size 可选 int 每页数量
 * @param status 可选 int 状态筛选
 * @param position 可选 int 位置筛选
 * @return {"code":0,"msg":"success","data":{"list":[],"total":0,"current_page":1,"per_page":50,"position_list":[],"status_list":[]}}
 * @number 16
 */
func callGetAdList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Ad GetAdList")

	// 解析请求参数
	page := gconv.Int32(r.GetQuery("page", "1"))
	size := gconv.Int32(r.GetQuery("size", "50"))
	status := gconv.Int32(r.GetQuery("status", "-1"))
	position := gconv.Int32(r.GetQuery("position", "0"))

	req := &v9.GetAdListReq{
		Page:     page,
		Size:     size,
		Status:   status,
		Position: position,
	}

	// 调用gRPC服务
	client := v9.NewAdClient(conn)
	res, err := client.GetAdList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Ad GetAdList failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/广告管理
 * @title 创建广告
 * @description 创建广告的接口
 * @method post
 * @url /api/backend/app/create-ad
 * @param title 必选 string 广告标题
 * @param image 必选 string 广告图片
 * @param link 必选 string 广告链接
 * @param position 必选 int 广告位置
 * @param status 必选 int 广告状态
 * @param sort 必选 int 排序
 * @return {"code":0,"msg":"success","data":{}}
 * @number 17
 */
func callCreateAd(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Ad CreateAd")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	// 验证必填参数
	if err := validateCreateAdRequest(requestData); err != nil {
		return err
	}

	req := &v9.CreateAdReq{
		Title:       gconv.String(requestData["title"]),
		Image:       gconv.String(requestData["image"]),
		Link:        gconv.String(requestData["link"]),
		Position:    gconv.Int32(requestData["position"]),
		Status:      gconv.Int32(requestData["status"]),
		Sort:        gconv.Int32(requestData["sort"]),
		StartTime:   gconv.String(requestData["start_time"]),
		ExpiredTime: gconv.String(requestData["expired_time"]),
	}

	// 调用gRPC服务
	client := v9.NewAdClient(conn)
	res, err := client.CreateAd(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Ad CreateAd failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

// validateCreateAdRequest 验证创建广告请求参数
func validateCreateAdRequest(reqData map[string]interface{}) error {
	// 验证标题
	title, ok := reqData["title"].(string)
	if !ok || title == "" {
		return gerror.New("请输入广告标题")
	}
	if len(title) < 2 || len(title) > 100 {
		return gerror.New("广告标题长度必须在2-100个字符之间")
	}

	// 验证图片
	image, ok := reqData["image"].(string)
	if !ok || image == "" {
		return gerror.New("请上传广告图片")
	}

	// 验证位置
	position, ok := reqData["position"]
	if !ok {
		return gerror.New("请选择广告位置")
	}
	posInt := gconv.Int(position)
	if posInt < 1 || posInt > 5 {
		return gerror.New("广告位置必须在1-5之间")
	}

	// 验证状态
	status, ok := reqData["status"]
	if !ok {
		return gerror.New("请选择广告状态")
	}
	statusInt := gconv.Int(status)
	if statusInt != 0 && statusInt != 1 {
		return gerror.New("广告状态必须为0或1")
	}

	// 验证排序
	sort, ok := reqData["sort"]
	if !ok {
		return fmt.Errorf("请输入排序值")
	}
	sortInt := gconv.Int(sort)
	if sortInt < 0 {
		return gerror.New("排序值不能小于0")
	}

	// 验证开始时间
	startTime, ok := reqData["start_time"].(string)
	if !ok || startTime == "" {
		return gerror.New("请选择开始时间")
	}

	// 验证过期时间
	expiredTime, ok := reqData["expired_time"].(string)
	if !ok || expiredTime == "" {
		return gerror.New("请选择过期时间")
	}

	return nil
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

/**
 * showdoc
 * @catalog 后台/运营/广告管理
 * @title 更新广告
 * @description 更新广告的接口
 * @method post
 * @url /api/backend/app/update-ad
 * @param id 必选 int 广告ID
 * @param title 必选 string 广告标题
 * @param image 必选 string 广告图片
 * @param link 必选 string 广告链接
 * @param position 必选 int 广告位置
 * @param status 必选 int 广告状态
 * @param sort 必选 int 排序
 * @return {"code":0,"msg":"success","data":{}}
 * @number 18
 */
func callUpdateAd(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Ad UpdateAd")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	req := &v9.UpdateAdReq{
		Id:          gconv.Int32(requestData["id"]),
		Title:       gconv.String(requestData["title"]),
		Image:       gconv.String(requestData["image"]),
		Link:        gconv.String(requestData["link"]),
		Position:    gconv.Int32(requestData["position"]),
		Status:      gconv.Int32(requestData["status"]),
		Sort:        gconv.Int32(requestData["sort"]),
		StartTime:   gconv.String(requestData["start_time"]),
		ExpiredTime: gconv.String(requestData["expired_time"]),
	}

	// 调用gRPC服务
	client := v9.NewAdClient(conn)
	res, err := client.UpdateAd(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Ad UpdateAd failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/广告管理
 * @title 删除广告
 * @description 删除广告的接口
 * @method post
 * @url /api/backend/app/delete-ad
 * @param id 必选 int 广告ID
 * @return {"code":0,"msg":"success","data":{}}
 * @number 19
 */
func callDeleteAd(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Ad DeleteAd")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	req := &v9.DeleteAdReq{
		Id: gconv.Int32(requestData["id"]),
	}

	// 调用gRPC服务
	client := v9.NewAdClient(conn)
	res, err := client.DeleteAd(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Ad DeleteAd failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/公告管理
 * @title 获取公告列表
 * @description 获取公告列表的接口
 * @method get
 * @url /api/backend/app/notices
 * @param page 可选 int 页码
 * @param size 可选 int 每页数量
 * @param status 可选 int 状态筛选
 * @param type 可选 int 类型筛选
 * @return {"code":0,"msg":"success","data":{"list":[],"total":0,"current_page":1,"per_page":50,"type_list":[],"status_list":[]}}
 * @number 20
 */
func callGetNoticeList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Notice GetNoticeList")

	// 解析请求参数
	page := gconv.Int32(r.GetQuery("page", "1"))
	size := gconv.Int32(r.GetQuery("size", "50"))
	status := gconv.Int32(r.GetQuery("status", "-1"))
	noticeType := gconv.Int32(r.GetQuery("type", "0"))

	req := &v10.GetNoticeListReq{
		Page:   page,
		Size:   size,
		Status: status,
		Type:   noticeType,
	}

	// 调用gRPC服务
	client := v10.NewNoticeClient(conn)
	res, err := client.GetNoticeList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Notice GetNoticeList failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/公告管理
 * @title 创建公告
 * @description 创建公告的接口
 * @method post
 * @url /api/backend/app/create-notice
 * @param title 必选 string 公告标题
 * @param content 必选 string 公告内容
 * @param type 必选 int 公告类型
 * @param status 必选 int 公告状态
 * @param sort 必选 int 排序
 * @param is_top 必选 int 是否置顶
 * @return {"code":0,"msg":"success","data":{}}
 * @number 21
 */
func callCreateNotice(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Notice CreateNotice")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	req := &v10.CreateNoticeReq{
		Title:       gconv.String(requestData["title"]),
		Content:     gconv.String(requestData["content"]),
		Type:        gconv.Int32(requestData["type"]),
		Status:      gconv.Int32(requestData["status"]),
		Sort:        gconv.Int32(requestData["sort"]),
		IsTop:       gconv.Int32(requestData["is_top"]),
		StartTime:   gconv.String(requestData["start_time"]),
		ExpiredTime: gconv.String(requestData["expired_time"]),
		Url:         gconv.String(requestData["url"]),
	}

	// 调用gRPC服务
	client := v10.NewNoticeClient(conn)
	res, err := client.CreateNotice(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Notice CreateNotice failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/公告管理
 * @title 更新公告
 * @description 更新公告的接口
 * @method post
 * @url /api/backend/app/update-notice
 * @param id 必选 int 公告ID
 * @param title 必选 string 公告标题
 * @param content 必选 string 公告内容
 * @param type 必选 int 公告类型
 * @param status 必选 int 公告状态
 * @param sort 必选 int 排序
 * @param is_top 必选 int 是否置顶
 * @return {"code":0,"msg":"success","data":{}}
 * @number 22
 */
func callUpdateNotice(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Notice UpdateNotice")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	req := &v10.UpdateNoticeReq{
		Id:          gconv.Int32(requestData["id"]),
		Title:       gconv.String(requestData["title"]),
		Content:     gconv.String(requestData["content"]),
		Type:        gconv.Int32(requestData["type"]),
		Status:      gconv.Int32(requestData["status"]),
		Sort:        gconv.Int32(requestData["sort"]),
		IsTop:       gconv.Int32(requestData["is_top"]),
		StartTime:   gconv.String(requestData["start_time"]),
		ExpiredTime: gconv.String(requestData["expired_time"]),
		Url:         gconv.String(requestData["url"]),
	}

	// 调用gRPC服务
	client := v10.NewNoticeClient(conn)
	res, err := client.UpdateNotice(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Notice UpdateNotice failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/运营/公告管理
 * @title 删除公告
 * @description 删除公告的接口
 * @method post
 * @url /api/backend/app/delete-notice
 * @param id 必选 int 公告ID
 * @return {"code":0,"msg":"success","data":{}}
 * @number 23
 */
func callDeleteNotice(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Notice DeleteNotice")

	// 解析请求参数
	requestData := r.Context().Value("requestData").(map[string]interface{})

	req := &v10.DeleteNoticeReq{
		Id: gconv.Int32(requestData["id"]),
	}

	// 调用gRPC服务
	client := v10.NewNoticeClient(conn)
	res, err := client.DeleteNotice(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Notice DeleteNotice failed: %v", err)
		return err
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/公共
 * @title 获取会员等级列表
 * @description 获取会员等级选项列表的接口
 * @method get
 * @url /api/backend/app/options-user-grade
 * @return {"code":0,"msg":"success","data":[{"id":1,"name":"VIP1","sort":1,"status":1},{"id":2,"name":"VIP2","sort":2,"status":1}]}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data array 会员等级列表
 * @number 24
 */
func callGetUserGradeOptions(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Option GetUserGradeList")

	// 创建 gRPC 客户端
	client := v11.NewOptionClient(conn)
	req := &v11.UserGradeListRequest{}

	// 调用 gRPC 服务
	res, err := client.GetUserGradeList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Option GetUserGradeList failed: %v", err)
		util.WriteInternalError(r, "获取会员等级列表失败，请稍后重试")
		return nil
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/公共
 * @title 获取后台角色列表
 * @description 获取后台角色选项列表的接口
 * @method get
 * @url /api/backend/app/options-admin-role
 * @return {"code":0,"msg":"success","data":[{"id":1,"name":"超级管理员","description":"拥有所有权限","status":1},{"id":2,"name":"管理员","description":"拥有部分管理权限","status":1}]}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data array 后台角色列表
 * @number 25
 */
func callGetAdminRoleOptions(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Option GetAdminRoleList")

	// 创建 gRPC 客户端
	client := v11.NewOptionClient(conn)
	req := &v11.AdminRoleListRequest{}

	// 调用 gRPC 服务
	res, err := client.GetAdminRoleList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Option GetAdminRoleList failed: %v", err)
		util.WriteInternalError(r, "获取后台角色列表失败，请稍后重试")
		return nil
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/公共
 * @title 获取交易类型列表
 * @description 获取交易类型选项列表的接口
 * @method get
 * @url /api/backend/balance/options-change
 * @return {"code":0,"msg":"success","data":{"1":"转入","2":"充值","3":"红利","4":"返水","5":"转出","6":"提现成功","7":"提现返回","8":"提现冻结","9":"丢失补款","10":"多出扣款"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 交易类型列表
 * @number 26
 */
func callGetChangeOptions(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetChangeList")

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetChangeListReq{}

	// 调用 gRPC 服务
	res, err := client.GetChangeList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Balance GetChangeList failed: %v", err)
		util.WriteInternalError(r, "获取交易类型列表失败，请稍后重试")
		return nil
	}

	// 返回响应
	return writeProtobufResponse(ctx, r, res)
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

/**
 * showdoc
 * @catalog 总后台/商户管理
 * @title 获取商户列表
 * @description 获取商户列表的接口
 * @method get
 * @url /api/manage/customer/list
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"code":"SITE001","name":"测试商户","status":1,"created_at":"2024-01-01 12:00:00","updated_at":"2024-01-01 12:00:00"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func callGetCustomerList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer GetCustomerList")

	client := v12.NewCustomerClient(conn)
	req := &v12.GetCustomerListReq{}

	res, err := client.GetCustomerList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer GetCustomerList failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/商户管理
 * @title 更新商户
 * @description 更新商户信息的接口
 * @method post
 * @url /api/manage/customer/update
 * @param id 必选 int 商户ID
 * @param name 可选 string 商户名称
 * @param status 可选 int 商户状态
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 2
 */
func callUpdateCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer UpdateCustomer")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	// 验证必填参数
	if _, exists := reqData["id"]; !exists {
		util.WriteBadRequest(r, "缺少id参数")
		return fmt.Errorf("missing id parameter")
	}

	client := v12.NewCustomerClient(conn)
	req := &v12.UpdateCustomerReq{
		Id:     gconv.Int32(reqData["id"]),
		Name:   gconv.String(reqData["name"]),
		Status: gconv.Int32(reqData["status"]),
	}

	res, err := client.UpdateCustomer(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer UpdateCustomer failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/商户管理
 * @title 删除商户
 * @description 删除商户的接口
 * @method post
 * @url /api/manage/customer/delete
 * @param id 必选 int 商户ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 3
 */
func callDeleteCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer DeleteCustomer")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	// 验证必填参数
	if _, exists := reqData["id"]; !exists {
		util.WriteBadRequest(r, "缺少id参数")
		return fmt.Errorf("missing id parameter")
	}

	client := v12.NewCustomerClient(conn)
	req := &v12.DeleteCustomerReq{
		Id: gconv.Int32(reqData["id"]),
	}

	res, err := client.DeleteCustomer(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer DeleteCustomer failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/商户管理
 * @title 同步商户
 * @description 同步商户数据的接口
 * @method post
 * @url /api/manage/customer/sync
 * @param data 必选 array 商户数据数组
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 4
 */
func callSyncCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer SyncCustomer")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	// 解析 data 数组
	dataInterface, exists := reqData["data"]
	if !exists {
		util.WriteBadRequest(r, "缺少data参数")
		return fmt.Errorf("missing data parameter")
	}

	var customers []*v12.CustomerItem

	// 转换数据格式
	switch dataValue := dataInterface.(type) {
	case []interface{}:
		for _, item := range dataValue {
			if customerMap, ok := item.(map[string]interface{}); ok {
				customer := &v12.CustomerItem{
					Id:        gconv.Int32(customerMap["id"]),
					Code:      gconv.String(customerMap["code"]),
					Name:      gconv.String(customerMap["name"]),
					Status:    gconv.Int32(customerMap["status"]),
					CreatedAt: gconv.String(customerMap["created_at"]),
					UpdatedAt: gconv.String(customerMap["updated_at"]),
				}
				customers = append(customers, customer)
			}
		}
	default:
		util.WriteBadRequest(r, "data参数格式错误")
		return fmt.Errorf("invalid data parameter format")
	}

	client := v12.NewCustomerClient(conn)
	req := &v12.SyncCustomerReq{
		Data: customers,
	}

	res, err := client.SyncCustomer(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer SyncCustomer failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/管理员认证
 * @title 管理员登录
 * @description 管理员登录的接口
 * @method post
 * @url /api/manage/admin/login
 * @param username 必选 string 用户名
 * @param password 必选 string 密码
 * @return {"code":0,"msg":"success","data":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...","message":"登录成功"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func callManageAdminLogin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Manage Admin Login")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	// 验证必填参数
	username := gconv.String(reqData["username"])
	password := gconv.String(reqData["password"])

	if username == "" {
		util.WriteBadRequest(r, "用户名不能为空")
		return fmt.Errorf("missing username parameter")
	}

	if password == "" {
		util.WriteBadRequest(r, "密码不能为空")
		return fmt.Errorf("missing password parameter")
	}

	client := v13.NewAdminClient(conn)
	req := &v13.ManageLoginReq{
		Username: username,
		Password: password,
	}

	res, err := client.Login(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Manage Admin Login failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/管理员认证
 * @title 刷新Token
 * @description 刷新管理员Token的接口
 * @method get
 * @url /api/manage/admin/refresh-token
 * @return {"code":0,"msg":"success","data":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 2
 */
func callManageAdminRefreshToken(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Manage Admin RefreshToken")

	client := v13.NewAdminClient(conn)
	req := &v13.ManageRefreshTokenReq{}

	res, err := client.RefreshToken(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Manage Admin RefreshToken failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/管理员认证
 * @title 获取管理员信息
 * @description 获取当前登录管理员信息的接口
 * @method get
 * @url /api/manage/admin/get-info
 * @return {"code":0,"msg":"success","data":{"roles":["admin","super_admin"],"name":"管理员","avatar":"https://wpimg.wallstcn.com/f778738c-e4f8-4870-b634-56703b4acafe.gif","introduction":"我是超级管理员"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 3
 */
func callManageAdminGetInfo(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Manage Admin GetInfo")

	client := v13.NewAdminClient(conn)
	req := &v13.ManageGetInfoReq{}

	res, err := client.GetInfo(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Manage Admin GetInfo failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/管理员认证
 * @title 管理员退出登录
 * @description 管理员退出登录的接口
 * @method post
 * @url /api/manage/admin/logout
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"退出成功"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 4
 */
func callManageAdminLogout(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Manage Admin Logout")

	client := v13.NewAdminClient(conn)
	req := &v13.ManageLogoutReq{}

	res, err := client.Logout(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Manage Admin Logout failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/管理员
 * @title 获取管理员菜单
 * @description 获取管理员菜单列表的接口
 * @method get
 * @url /api/manage/admin/menus
 * @header Authorization 必选 string Bearer token
 * @return {"code":200,"msg":"success","data":{"menus":[{"id":1,"name":"系统管理","path":"/system","type":1,"sort":0,"children":[]}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param menus array 菜单列表
 * @number 5
 */
func callManageAdminMenus(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling Manage Admin Menus")

	// 创建 gRPC 客户端
	client := v13.NewAdminClient(conn)
	req := &v13.ManageMenusReq{}

	// 调用 gRPC 服务
	res, err := client.Menus(ctx, req)
	if err != nil {
		// 根据gRPC错误类型返回不同的HTTP状态码
		if strings.Contains(err.Error(), "未登录或登录已过期") ||
			strings.Contains(err.Error(), "管理员不存在") ||
			strings.Contains(err.Error(), "账号已被禁用") {
			util.WriteUnauthorized(r, "登录已过期，请重新登录")
		} else {
			util.WriteInternalError(r, "获取菜单列表失败，请稍后重试")
		}
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

// getAdminIdFromRequest 从请求中获取管理员ID
func getAdminIdFromRequest(r *ghttp.Request) uint {
	token := r.Header.Get("Authorization")
	if token == "" {
		return 0
	}
	return 1
}

// writeJSONResponse 写入JSON响应
func writeJSONResponse(ctx context.Context, r *ghttp.Request, data interface{}) error {
	util.LogWithTrace(ctx, "info", "=== writeJSONResponse 开始 ===")

	// 确保响应头正确设置
	r.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	r.Response.Header().Set("Cache-Control", "no-cache")

	util.LogWithTrace(ctx, "info", "=== 响应头设置完成，开始序列化 ===")
	// 序列化数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		util.LogWithTrace(ctx, "error", "序列化JSON响应失败: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "=== 序列化完成，准备写入响应，数据长度: %d ===", len(jsonData))
	util.LogWithTrace(ctx, "info", "=== 响应数据前100字符: %s ===", string(jsonData[:min(100, len(jsonData))]))

	// 设置状态码
	r.Response.WriteStatus(200)
	util.LogWithTrace(ctx, "info", "=== 状态码设置完成，开始写入数据 ===")

	// 写入响应数据
	r.Response.Write(jsonData)
	util.LogWithTrace(ctx, "info", "=== 数据写入完成 ===")

	// 不调用ExitAll，让请求正常完成
	util.LogWithTrace(ctx, "info", "=== writeJSONResponse 完成 ===")
	return nil
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
