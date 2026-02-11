package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/util/gconv"
	v16 "jh_gateway/api/frontend/user/v1"
	"regexp"
	"strings"

	v15 "jh_gateway/api/frontend/public/v1"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// writeProtobufResponse 统一处理protobuf响应的JSON序列化
func writeProtobufResponse(ctx context.Context, r *ghttp.Request, pbMessage proto.Message) error {
	marshaler := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}

	jsonData, err := marshaler.Marshal(pbMessage)
	if err != nil {
		util.LogWithTrace(ctx, "error", "protobuf JSON marshal failed: %v", err)
		util.WriteInternalError(r, "数据序列化失败")
		return err
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(jsonData, &responseData); err != nil {
		util.LogWithTrace(ctx, "error", "JSON parse to map failed: %v", err)
		util.WriteSuccess(r, pbMessage)
		return nil
	}

	normalizeMessageFields(responseData, r.URL.Path)

	util.WriteSuccess(r, responseData)
	return nil
}

// normalizeMessageFields 递归翻译 msg/message 字段（仅前台路径生效）
func normalizeMessageFields(value interface{}, path string) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if key == "msg" || key == "message" {
				if s, ok := val.(string); ok {
					v[key] = util.NormalizeMessageWithPath(s, path)
					continue
				}
			}
			normalizeMessageFields(val, path)
		}
	case []interface{}:
		for i := range v {
			normalizeMessageFields(v[i], path)
		}
	}
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
func CallQuickRegister(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
		util.WriteInternalError(r, "注册服务暂时不可用，请稍后重试")
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
func CallUserRegister(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
	inviteCode, _ := reqData["invite_code"].(string)

	if username == "" {
		util.WriteBadRequest(r, "用户名必填")
		return nil
	}
	if password == "" {
		util.WriteBadRequest(r, "密码必填")
		return nil
	}
	if usernameLen := len(strings.TrimSpace(username)); usernameLen < 6 || usernameLen > 16 {
		util.WriteBadRequest(r, "用户名长度需为6-16位")
		return nil
	}
	if passwordLen := len(password); passwordLen < 6 || passwordLen > 20 {
		util.WriteBadRequest(r, "密码长度需为6-20位")
		return nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+$`).MatchString(username) {
		util.WriteBadRequest(r, "用户名仅允许字母和数字")
		return nil
	}
	if !regexp.MustCompile(`[A-Za-z]`).MatchString(username) || !regexp.MustCompile(`[0-9]`).MatchString(username) {
		util.WriteBadRequest(r, "用户名必须包含字母和数字")
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
		InviteCode:  inviteCode,
	}

	// 调用 gRPC 服务
	res, err := client.Register(ctx, req)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "验证码") {
			util.WriteForbidden(r, errMsg)
		} else if strings.Contains(errMsg, "用户名必须包含字母和数字") {
			util.WriteBadRequest(r, errMsg)
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
func CallUserLogin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGenerateCaptcha(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallRefreshToken(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetUserInfo(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallUserLogout(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallBindEmail(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallBindMobile(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
		"message": util.NormalizeMessageWithPath(res.Message, r.URL.Path),
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
func CallChangeLoginPassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallBindLoginPassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetAppGradeList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetAppNoticeList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetAppAdList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetSiteSetting(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
 * @catalog 前台/系统/活动配置
 * @title 获取活动列表
 * @description 获取 site_activity 表数据（前台）
 * @method get
 * @url /api/frontend/app/site-activity-list
 * @param token 必选 string token
 * @param site_id 可选 int 站点id
 * @param activity_id 可选 int 活动ID
 * @param code 可选 string 活动编码
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"site_code":"jh","activity_id":1,"name":"邀请好友活动","code":"INVITE","description":"","start_time":"2025-12-29 08:00:00","end_time":"2026-11-28 08:00:00","fixed_params":"{}","status":1,"sort":0,"table":"activity_invite_bonus","created_at":"2026-02-06 07:59:33","updated_at":"2026-02-06 07:59:33","uri":"/invite_activity"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 120
 */
func CallGetSiteActivityList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Public GetSiteActivityList")

	client := v15.NewPublicClient(conn)
	siteId := r.Get("site_id", 0).Int32()
	if siteId <= 0 {
		if ctxVar := r.GetCtxVar("site_id"); ctxVar != nil {
			siteId = ctxVar.Int32()
		}
	}
	req := &v15.GetSiteActivityListReq{
		SiteId:     siteId,
		ActivityId: r.Get("activity_id", 0).Int32(),
		Code:       r.Get("code", "").String(),
	}

	res, err := client.GetSiteActivityList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取活动列表失败，请稍后重试")
		return nil
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK设置支付密码
 * @description 设置支付密码接口
 * @method post
 * @url /api/frontend/app/set-pay-password
 * @header Authorization 必选 string Bearer {token}
 * @param pay_password 必选 string 支付密码（6位数字）
 * @return {"code":0,"msg":"success","data":{"message":"设置支付密码成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.message string 提示消息
 * @remark 支付密码必须是6位数字
 * @number 120
 */
func CallSetPayPassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC SetPayPassword")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteInternalError(r, "解析请求数据失败")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.SetPayPasswordReq{
		PayPassword: gconv.String(reqData["pay_password"]),
	}

	res, err := client.SetPayPassword(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "未登录") {
			util.WriteUnauthorized(r, "请先登录")
		} else if strings.Contains(err.Error(), "6位数字") {
			util.WriteBadRequest(r, "支付密码必须是6位数字")
		} else {
			util.WriteInternalError(r, "设置支付密码失败")
		}
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
 * @title OK验证支付密码
 * @description 验证支付密码是否正确
 * @method post
 * @url /api/frontend/app/verify-pay-password
 * @header Authorization 必选 string Bearer {token}
 * @param pay_password 必选 string 支付密码
 * @return {"code":0,"msg":"success","data":{"is_valid":true,"message":"验证成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.is_valid bool 是否正确
 * @return_param data.message string 提示消息
 * @remark 用于提现等操作前验证支付密码
 * @number 121
 */
func CallVerifyPayPassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC VerifyPayPassword")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteInternalError(r, "解析请求数据失败")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.VerifyPayPasswordReq{
		PayPassword: gconv.String(reqData["pay_password"]),
	}

	res, err := client.VerifyPayPassword(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "未登录") {
			util.WriteUnauthorized(r, "请先登录")
		} else {
			util.WriteInternalError(r, "验证支付密码失败")
		}
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"is_valid": res.IsValid,
		"message":  res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK修改支付密码
 * @description 修改支付密码
 * @method post
 * @url /api/frontend/app/change-pay-password
 * @header Authorization 必选 string Bearer {token}
 * @param old_pay_password 必选 string 原支付密码
 * @param new_pay_password 必选 string 新支付密码
 * @return {"code":0,"msg":"success","data":{"message":"修改支付密码成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.message string 提示消息
 * @remark 支付密码必须是6位数字，新旧密码不能相同
 * @number 122
 */
func CallChangePayPassword(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ChangePayPassword")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteInternalError(r, "解析请求数据失败")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.ChangePayPasswordReq{
		OldPayPassword: gconv.String(reqData["old_pay_password"]),
		NewPayPassword: gconv.String(reqData["new_pay_password"]),
	}

	res, err := client.ChangePayPassword(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "未登录") {
			util.WriteUnauthorized(r, "请先登录")
		} else if strings.Contains(err.Error(), "6位数字") {
			util.WriteBadRequest(r, err.Error())
		} else if strings.Contains(err.Error(), "不能相同") {
			util.WriteBadRequest(r, err.Error())
		} else if strings.Contains(err.Error(), "原支付密码错误") {
			util.WriteBadRequest(r, "原支付密码错误")
		} else if strings.Contains(err.Error(), "未设置支付密码") {
			util.WriteBadRequest(r, "未设置支付密码，请先设置支付密码")
		} else {
			util.WriteInternalError(r, "修改支付密码失败")
		}
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
 * @title OK获取邀请信息
 * @description 获取我的邀请码、邀请链接、邀请统计信息
 * @method get
 * @url /api/frontend/app/invite-info
 * @header Authorization 必选 string Bearer {token}
 * @return {"code":0,"msg":"success","data":{"invite_code":"123","invite_url":"https://example.com/register?invite_code=123","total_invites":10,"today_invites":2,"total_reward":"100.00"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.invite_code string 邀请码
 * @return_param data.invite_url string 邀请链接
 * @return_param data.total_invites int 总邀请人数
 * @return_param data.today_invites int 今日邀请人数
 * @return_param data.total_reward string 总奖励金额
 * @remark 需要登录后访问
 * @number 123
 */
func CallGetInviteInfo(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetInviteInfo")

	client := v16.NewUserClient(conn)
	req := &v16.GetInviteInfoReq{}

	res, err := client.GetInviteInfo(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "未登录") {
			util.WriteUnauthorized(r, "请先登录")
		} else {
			util.WriteInternalError(r, "获取邀请信息失败")
		}
		return nil
	}

	// 返回成功响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title OK获取邀请列表
 * @description 获取我邀请的用户列表
 * @method get
 * @url /api/frontend/app/invite-list?page=1&page_size=20
 * @header Authorization 必选 string Bearer {token}
 * @param page 可选 int 页码，默认1
 * @param page_size 可选 int 每页数量，默认20，最大100
 * @return {"code":0,"msg":"success","data":{"list":[{"user_id":123,"username":"test001","register_time":"2025-01-19 10:00:00","total_bet":"1000.00","total_recharge":"500.00","status":1}],"total":10,"page":1,"page_size":20}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.list array 邀请用户列表
 * @return_param data.list.user_id int 用户ID
 * @return_param data.list.username string 用户名
 * @return_param data.list.register_time string 注册时间
 * @return_param data.list.total_bet string 总投注额
 * @return_param data.list.total_recharge string 总充值额
 * @return_param data.list.status int 状态 1=正常 2=禁用
 * @return_param data.total int 总数
 * @return_param data.page int 当前页码
 * @return_param data.page_size int 每页数量
 * @remark 需要登录后访问
 * @number 124
 */
func CallGetInviteList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetInviteList")

	client := v16.NewUserClient(conn)
	req := &v16.GetInviteListReq{
		Page:     r.Get("page", 1).Int32(),
		PageSize: r.Get("page_size", 20).Int32(),
	}

	res, err := client.GetInviteList(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "未登录") {
			util.WriteUnauthorized(r, "请先登录")
		} else {
			util.WriteInternalError(r, "获取邀请列表失败")
		}
		return nil
	}

	// 返回成功响应
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title 获取用户消息
 * @description 获取当前用户消息列表
 * @method get
 * @url /api/frontend/app/app-user-messages
 * @param page 可选 int 页码
 * @param size 可选 int 每页数量
 * @return {"code":0,"msg":"success","data":{"list":[],"total":0,"current_page":1,"per_page":20,"unread_message_count":0}}
 * @number 120
 */
func CallGetUserMessages(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	page := gconv.Int32(r.GetQuery("page", "1"))
	size := gconv.Int32(r.GetQuery("size", "20"))

	client := v16.NewUserClient(conn)
	req := &v16.GetUserMessagesReq{
		Page: page,
		Size: size,
	}

	res, err := client.GetUserMessages(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取用户消息失败")
		return nil
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title 获取未读数量
 * @description 获取当前用户未读消息数量
 * @method get
 * @url /api/frontend/app/app-unread-count
 * @return {"code":0,"msg":"success","data":{"count":0}}
 * @number 121
 */
func CallGetUnreadCount(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	client := v16.NewUserClient(conn)
	req := &v16.GetUnreadCountReq{}
	res, err := client.GetUnreadCount(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取未读消息数量失败")
		return nil
	}
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/用户
 * @title 阅读消息
 * @description 标记消息已读
 * @method post
 * @url /api/frontend/app/app-read-message
 * @param message_id 必选 int 消息ID
 * @return {"code":0,"msg":"success","data":{"message":"success"}}
 * @number 122
 */
func CallReadMessage(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据格式错误")
		return nil
	}

	messageId := gconv.Int32(reqData["message_id"])
	if messageId <= 0 {
		messageId = gconv.Int32(r.Get("message_id"))
	}
	if messageId <= 0 {
		util.WriteBadRequest(r, "message_id必填")
		return nil
	}

	client := v16.NewUserClient(conn)
	req := &v16.ReadMessageReq{
		MessageId: messageId,
	}
	res, err := client.ReadMessage(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "读取消息失败")
		return nil
	}

	return writeProtobufResponse(ctx, r, res)
}
