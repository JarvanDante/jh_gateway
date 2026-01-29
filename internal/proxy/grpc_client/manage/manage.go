package manage

import (
	"context"
	"encoding/json"
	"fmt"
	v12 "jh_gateway/api/manage/customer/v1"
	v18 "jh_gateway/api/manage/game/v1"
	v14 "jh_gateway/api/manage/option/v1"
	v20 "jh_gateway/api/manage/payment/v1"
	v19 "jh_gateway/api/manage/site_game/v1"
	v21 "jh_gateway/api/manage/site_payment/v1"
	"strings"

	v13 "jh_gateway/api/manage/admin/v1"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
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

	util.WriteSuccess(r, responseData)
	return nil
}

// 辅助函数
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
			return gconv.Int32(v)
		}
	}
	return 0
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
func CallManageAdminLogin(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
		return nil
	}

	if password == "" {
		util.WriteBadRequest(r, "密码不能为空")
		return nil
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
func CallManageAdminRefreshToken(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallManageAdminGetInfo(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallManageAdminLogout(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallManageAdminMenus(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetCustomerList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
 * @title 获取商户详情
 * @description 获取指定商户详情的接口
 * @method get
 * @url /api/manage/customer/detail/{id}
 * @param id 必选 int 商户ID
 * @return {"code":0,"msg":"success","data":{"id":1,"code":"SITE001","name":"测试商户","kv_config":"","status":1,"is_sync":1,"is_config":1,"created_at":"2024-01-01 12:00:00","updated_at":"2024-01-01 12:00:00"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 商户详情数据
 * @number 1.5
 */
func CallGetCustomerDetail(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer GetCustomerDetail")

	// 从URL路径中获取ID参数
	idStr := r.Get("id").String()
	if idStr == "" {
		util.WriteBadRequest(r, "商户ID不能为空")
		return fmt.Errorf("商户ID不能为空")
	}

	id := r.Get("id").Int32()
	if id <= 0 {
		util.WriteBadRequest(r, "商户ID必须大于0")
		return fmt.Errorf("商户ID必须大于0")
	}

	client := v12.NewCustomerClient(conn)
	req := &v12.GetCustomerDetailReq{
		Id: id,
	}

	res, err := client.GetCustomerDetail(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer GetCustomerDetail failed: %v", err)
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
func CallUpdateCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
		Id:            gconv.Int32(reqData["id"]),
		Name:          gconv.String(reqData["name"]),
		Timezone:      gconv.String(reqData["timezone"]),
		Currency:      gconv.String(reqData["currency"]),
		Status:        gconv.Int32(reqData["status"]),
		KvConfig:      gconv.String(reqData["kv_config"]),
		DbLinkSite:    gconv.String(reqData["db_link_site"]),
		DbLinkBalance: gconv.String(reqData["db_link_balance"]),
		DbLinkGame:    gconv.String(reqData["db_link_game"]),
		IsSync:        gconv.Int32(reqData["is_sync"]),
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
func CallDeleteCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallSyncCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
					Timezone:  gconv.String(customerMap["timezone"]),
					Currency:  gconv.String(customerMap["currency"]),
					Status:    gconv.Int32(customerMap["status"]),
					KvConfig:  gconv.String(customerMap["kv_config"]),
					IsSync:    gconv.Int32(customerMap["is_sync"]),
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
 * @catalog 总后台/商户管理
 * @title 发布商户配置
 * @description 发布商户配置到Consul KV的接口
 * @method post
 * @url /api/manage/customer/publish
 * @param id 必选 int 商户ID
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"发布成功"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.success bool 发布是否成功
 * @return_param data.message string 发布结果消息
 * @number 5
 */
func CallPublishCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer PublishCustomer")

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

	id := gconv.Int32(reqData["id"])
	if id <= 0 {
		util.WriteBadRequest(r, "商户ID必须大于0")
		return fmt.Errorf("商户ID必须大于0")
	}

	client := v12.NewCustomerClient(conn)
	req := &v12.PublishCustomerReq{
		Id: id,
	}

	res, err := client.PublishCustomer(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer PublishCustomer failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/商户管理
 * @title 回溯发布配置
 * @description 回溯发布配置
 * @method post
 * @url /api/manage/customer/rollback
 * @param id 必选 int 商户ID
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"发布成功"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.success bool 发布是否成功
 * @return_param data.message string 发布结果消息
 * @number 5
 */
func CallRollbackCustomer(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer RollbackCustomer")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	id := gconv.Int32(reqData["id"])
	targetVersion := gconv.Uint64(reqData["target_version"])

	if id <= 0 {
		util.WriteBadRequest(r, "商户ID必须大于0")
		return fmt.Errorf("商户ID必须大于0")
	}

	if targetVersion <= 0 {
		util.WriteBadRequest(r, "目标版本号必须大于0")
		return fmt.Errorf("目标版本号必须大于0")
	}

	client := v12.NewCustomerClient(conn)
	req := &v12.RollbackCustomerReq{
		Id:            id,
		TargetVersion: targetVersion,
	}

	res, err := client.RollbackCustomer(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer RollbackCustomer failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/商户管理
 * @title 发布历史
 * @description 发布历史
 * @method post
 * @url /api/manage/customer/version-history
 * @param id 必选 int 商户ID
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"发布成功"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.success bool 发布是否成功
 * @return_param data.message string 发布结果消息
 * @number 5
 */
func CallGetCustomerVersionHistory(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Customer GetCustomerVersionHistory")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	siteCode := gconv.String(reqData["site_code"])
	if siteCode == "" {
		util.WriteBadRequest(r, "站点代码不能为空")
		return fmt.Errorf("站点代码不能为空")
	}

	client := v12.NewCustomerClient(conn)
	req := &v12.GetCustomerVersionHistoryReq{
		SiteCode: siteCode,
		Page:     gconv.Int32(reqData["page"]),
		Size:     gconv.Int32(reqData["size"]),
	}

	res, err := client.GetCustomerVersionHistory(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Customer GetCustomerVersionHistory failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/游戏管理
 * @title 获取游戏列表
 * @description 获取游戏列表的接口
 * @method get
 * @url /api/manage/game/list
 * @param type 可选 int 游戏类型 1=体育 2=彩票 3=真人 4=电子游戏
 * @param status 可选 int 状态 1=启用 0=禁用
 * @param platform 可选 string 平台标识
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认20
 * @return {"code":0,"msg":"success","data":{"list":[],"total":0}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func CallGetGameList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageGame GetGameList")

	client := v18.NewManageGameClient(conn)
	req := &v18.GetGameListReq{
		Type:     r.Get("type", 0).Int32(),
		Status:   r.Get("status", -1).Int32(),
		Platform: r.Get("platform", "").String(),
		Page:     r.Get("page", 1).Int32(),
		Size:     r.Get("size", 20).Int32(),
	}

	res, err := client.GetGameList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGameList failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/游戏管理
 * @title 获取游戏详情
 * @description 获取指定游戏详情的接口
 * @method get
 * @url /api/manage/game/detail/{id}
 * @param id 必选 int 游戏ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 游戏详情数据
 * @number 2
 */
func CallGetGameDetail(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageGame GetGameDetail")

	idStr := r.Get("id").String()
	if idStr == "" {
		util.WriteBadRequest(r, "游戏ID不能为空")
		return fmt.Errorf("游戏ID不能为空")
	}

	id := r.Get("id").Int32()
	if id <= 0 {
		util.WriteBadRequest(r, "游戏ID必须大于0")
		return fmt.Errorf("游戏ID必须大于0")
	}

	client := v18.NewManageGameClient(conn)
	req := &v18.GetGameDetailReq{
		Id: id,
	}

	res, err := client.GetGameDetail(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGameDetail failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/游戏管理
 * @title 创建游戏
 * @description 创建游戏的接口
 * @method post
 * @url /api/manage/game/create
 * @param type 必选 int 游戏类型 1=体育 2=彩票 3=真人 4=电子游戏
 * @param name 必选 string 游戏名称
 * @param platform 必选 string 平台标识
 * @param code 必选 string 游戏标识
 * @param image_code 可选 string 图片标识
 * @param status 可选 int 状态 1=启用 0=禁用
 * @param db_name 可选 string 数据表名称
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{"id":1}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.id int 创建的游戏ID
 * @number 3
 */
func CallCreateGame(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageGame CreateGame")

	client := v18.NewManageGameClient(conn)
	req := &v18.CreateGameReq{
		Type:      r.Get("type").Int32(),
		Name:      r.Get("name").String(),
		Platform:  r.Get("platform").String(),
		Code:      r.Get("code").String(),
		ImageCode: r.Get("image_code", "").String(),
		Status:    r.Get("status", 1).Int32(),
		DbName:    r.Get("db_name", "").String(),
		Remark:    r.Get("remark", "").String(),
		ApiId:     r.Get("api_id", "").String(),
		ApiKey:    r.Get("api_key", "").String(),
		ApiUrl:    r.Get("api_url", "").String(),
	}

	res, err := client.CreateGame(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game CreateGame failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/游戏管理
 * @title 更新游戏
 * @description 更新游戏信息的接口
 * @method post
 * @url /api/manage/game/update
 * @param id 必选 int 游戏ID
 * @param type 可选 int 游戏类型
 * @param name 可选 string 游戏名称
 * @param platform 可选 string 平台标识
 * @param code 可选 string 游戏标识
 * @param image_code 可选 string 图片标识
 * @param status 可选 int 状态
 * @param db_name 可选 string 数据表名称
 * @param remark 可选 string 备注
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 4
 */
func CallUpdateGame(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageGame UpdateGame")

	client := v18.NewManageGameClient(conn)
	req := &v18.UpdateGameReq{
		Id:        r.Get("id").Int32(),
		Type:      r.Get("type", 0).Int32(),
		Name:      r.Get("name", "").String(),
		Platform:  r.Get("platform", "").String(),
		Code:      r.Get("code", "").String(),
		ImageCode: r.Get("image_code", "").String(),
		Status:    r.Get("status", -1).Int32(),
		DbName:    r.Get("db_name", "").String(),
		Remark:    r.Get("remark", "").String(),
		ApiId:     r.Get("api_id", "").String(),
		ApiKey:    r.Get("api_key", "").String(),
		ApiUrl:    r.Get("api_url", "").String(),
	}

	res, err := client.UpdateGame(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game UpdateGame failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/游戏管理
 * @title 删除游戏
 * @description 删除游戏的接口
 * @method post
 * @url /api/manage/game/delete
 * @param id 必选 int 游戏ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 5
 */
func CallDeleteGame(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageGame DeleteGame")

	client := v18.NewManageGameClient(conn)
	req := &v18.DeleteGameReq{
		Id: r.Get("id").Int32(),
	}

	res, err := client.DeleteGame(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game DeleteGame failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/游戏管理
 * @title 补单
 * @description 补单接口，用于补充指定时间范围内的游戏注单
 * @method post
 * @url /api/manage/game/supplement-bet-records
 * @param platform 必选 string 游戏平台代码，如 "ONE"
 * @param start_time 必选 string 开始时间，格式: "2026-01-24 00:00:00"
 * @param end_time 必选 string 结束时间，格式: "2026-01-24 19:00:00"
 * @param site_codes 可选 array 站点代码列表，为空则所有站点
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"补单完成: 平台=ONE, 成功站点=1, 失败站点=0, 总记录数=100","total_records":100}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 补单结果消息
 * @return_param data.total_records int 总记录数
 * @number 6
 */
func CallSupplementBetRecords(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageGame SupplementBetRecords")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	// 验证必填参数
	platform := gconv.String(reqData["platform"])
	if platform == "" {
		util.WriteBadRequest(r, "游戏平台不能为空")
		return fmt.Errorf("platform is required")
	}

	startTime := gconv.String(reqData["start_time"])
	if startTime == "" {
		util.WriteBadRequest(r, "开始时间不能为空")
		return fmt.Errorf("start_time is required")
	}

	endTime := gconv.String(reqData["end_time"])
	if endTime == "" {
		util.WriteBadRequest(r, "结束时间不能为空")
		return fmt.Errorf("end_time is required")
	}

	// 可选参数：站点代码列表
	var siteCodes []string
	if siteCodesData, ok := reqData["site_codes"]; ok {
		siteCodes = gconv.Strings(siteCodesData)
	}

	client := v18.NewManageGameClient(conn)
	req := &v18.SupplementBetRecordsReq{
		Platform:  platform,
		StartTime: startTime,
		EndTime:   endTime,
		SiteCodes: siteCodes,
	}

	util.LogWithTrace(ctx, "info", "补单请求: platform=%s, start_time=%s, end_time=%s, site_codes=%v",
		platform, startTime, endTime, siteCodes)

	res, err := client.SupplementBetRecords(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game SupplementBetRecords failed: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "补单完成: success=%v, message=%s, total_records=%d",
		res.Success, res.Message, res.TotalRecords)

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/游戏管理
 * @title 获取同步任务列表
 * @description 获取游戏注单同步任务列表的接口
 * @method get
 * @url /api/manage/game/sync-tasks
 * @param site_id 可选 int 站点ID，0表示不过滤
 * @param site_code 可选 string 站点代码
 * @param platform 可选 string 游戏平台
 * @param task_type 可选 int 任务类型，0表示不过滤，1=定时拉单，2=手动补单
 * @param status 可选 int 任务状态，0表示不过滤，1=进行中，2=已完成，3=失败
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认20
 * @return {"code":0,"msg":"success","data":{"list":[],"total":0,"page":1,"size":20}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 任务列表
 * @return_param data.total int 总记录数
 * @return_param data.page int 当前页码
 * @return_param data.size int 每页数量
 * @number 7
 */
func CallManageGetSyncTasks(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageGame ManageGetSyncTasks")

	client := v18.NewManageGameClient(conn)
	req := &v18.ManageGetSyncTasksReq{
		SiteId:   r.Get("site_id").Int32(),
		SiteCode: r.Get("site_code").String(),
		Platform: r.Get("platform").String(),
		TaskType: r.Get("task_type").Int32(),
		Status:   r.Get("status").Int32(),
		Page:     r.Get("page", 1).Int32(),
		Size:     r.Get("size", 20).Int32(),
	}

	util.LogWithTrace(ctx, "info", "获取同步任务列表: site_id=%d, site_code=%s, platform=%s, task_type=%d, status=%d, page=%d, size=%d",
		req.SiteId, req.SiteCode, req.Platform, req.TaskType, req.Status, req.Page, req.Size)

	res, err := client.ManageGetSyncTasks(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC ManageGame ManageGetSyncTasks failed: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "获取同步任务列表成功: total=%d, page=%d, size=%d", res.Total, res.Page, res.Size)

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点游戏管理
 * @title 获取站点游戏列表
 * @description 获取指定站点的游戏列表
 * @method get
 * @url /api/manage/site_game/list
 * @param site_code 必选 string 站点编码
 * @return {"code":0,"msg":"success","data":{"list":[]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func CallGetSiteGameList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSiteGame GetSiteGameList")

	siteCode := r.Get("site_code").String()
	if siteCode == "" {
		util.WriteBadRequest(r, "站点编码不能为空")
		return fmt.Errorf("站点编码不能为空")
	}

	client := v19.NewManageSiteGameClient(conn)
	req := &v19.GetSiteGameListReq{
		SiteCode: siteCode,
	}

	res, err := client.GetSiteGameList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SiteGame GetSiteGameList failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点游戏管理
 * @title 更新站点游戏状态
 * @description 更新站点游戏的启用/禁用状态
 * @method post
 * @url /api/manage/site_game/update_status
 * @param id 必选 int 站点游戏ID
 * @param status 必选 int 状态 1=启用 0=禁用
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 2
 */
func CallUpdateSiteGameStatus(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSiteGame UpdateSiteGameStatus")

	client := v19.NewManageSiteGameClient(conn)
	req := &v19.UpdateSiteGameStatusReq{
		Id:     r.Get("id").Int32(),
		Status: r.Get("status").Int32(),
	}

	res, err := client.UpdateSiteGameStatus(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SiteGame UpdateSiteGameStatus failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点游戏管理
 * @title 绑定游戏到站点
 * @description 将游戏绑定到指定站点
 * @method post
 * @url /api/manage/site_game/bind
 * @param site_code 必选 string 站点编码
 * @param game_id 必选 int 游戏ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 3
 */
func CallBindSiteGame(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSiteGame BindSiteGame")

	client := v19.NewManageSiteGameClient(conn)
	req := &v19.BindSiteGameReq{
		SiteCode: r.Get("site_code").String(),
		GameId:   r.Get("game_id").Int32(),
	}

	res, err := client.BindSiteGame(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SiteGame BindSiteGame failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点游戏管理
 * @title 取消绑定游戏
 * @description 取消站点游戏绑定
 * @method post
 * @url /api/manage/site_game/unbind
 * @param id 必选 int 站点游戏ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 4
 */
func CallUnbindSiteGame(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSiteGame UnbindSiteGame")

	client := v19.NewManageSiteGameClient(conn)
	req := &v19.UnbindSiteGameReq{
		Id: r.Get("id").Int32(),
	}

	res, err := client.UnbindSiteGame(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SiteGame UnbindSiteGame failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/选项接口
 * @title 获取域名类型选项
 * @description 获取域名类型选项列表的接口
 * @method get
 * @url /api/manage/options/domain-type
 * @return {"code":0,"msg":"success","data":{"list":[{"value":1,"label":"API"},{"value":2,"label":"H5"},{"value":3,"label":"管理后台"},{"value":4,"label":"回调"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 选项列表
 * @return_param data.list.value int 选项值
 * @return_param data.list.label string 选项标签
 * @number 1
 */
func CallGetDomainTypeOptions(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageOption GetDomainTypeOptions")

	client := v14.NewManageOptionClient(conn)
	req := &v14.GetDomainTypeOptionsReq{}

	res, err := client.GetDomainTypeOptions(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC ManageOption GetDomainTypeOptions failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/选项接口
 * @title 获取商户选项
 * @description 获取商户选项
 * @method get
 * @url /api/manage/options/domain-type
 * @return {"code":0,"msg":"success","data":{"list":[{"value":1,"label":"API"},{"value":2,"label":"H5"},{"value":3,"label":"管理后台"},{"value":4,"label":"回调"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 选项列表
 * @return_param data.list.value int 选项值
 * @return_param data.list.label string 选项标签
 * @number 1
 */
func CallGetSiteOptions(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageOption GetSiteOptions")

	client := v14.NewManageOptionClient(conn)
	req := &v14.GetSiteOptionsReq{}

	res, err := client.GetSiteOptions(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC ManageOption GetSiteOptions failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/选项接口
 * @title 获取域名选项
 * @description 获取域名选项
 * @method get
 * @url /api/manage/options/domain-type
 * @return {"code":0,"msg":"success","data":{"list":[{"value":1,"label":"API"},{"value":2,"label":"H5"},{"value":3,"label":"管理后台"},{"value":4,"label":"回调"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 选项列表
 * @return_param data.list.value int 选项值
 * @return_param data.list.label string 选项标签
 * @number 1
 */
func CallGetSiteDomainList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageOption GetSiteDomainList")

	client := v14.NewManageOptionClient(conn)
	req := &v14.GetSiteDomainListReq{
		Page:       gconv.Int32(r.Get("page")),
		Size:       gconv.Int32(r.Get("size")),
		SiteCode:   r.Get("site_code").String(),
		DomainType: gconv.Int32(r.Get("domain_type")),
		Status:     gconv.Int32(r.Get("status")),
	}

	res, err := client.GetSiteDomainList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC ManageOption GetSiteDomainList failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点游戏管理
 * @title 创建站点域名
 * @description 创建域名选项
 * @method get
 * @url /api/manage/options/domain-type
 * @return {"code":0,"msg":"success","data":{"list":[{"value":1,"label":"API"},{"value":2,"label":"H5"},{"value":3,"label":"管理后台"},{"value":4,"label":"回调"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 选项列表
 * @return_param data.list.value int 选项值
 * @return_param data.list.label string 选项标签
 * @number 1
 */
func CallCreateSiteDomain(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageOption CreateSiteDomain")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v14.NewManageOptionClient(conn)
	req := &v14.CreateSiteDomainReq{
		SiteCode:   gconv.String(reqData["site_code"]),
		Domain:     gconv.String(reqData["domain"]),
		DomainType: gconv.Int32(reqData["domain_type"]),
		Status:     gconv.Int32(reqData["status"]),
	}

	res, err := client.CreateSiteDomain(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC ManageOption CreateSiteDomain failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点游戏管理
 * @title 更新站点域名
 * @description 更新站点域名
 * @method get
 * @url /api/manage/options/domain-type
 * @return {"code":0,"msg":"success","data":{"list":[{"value":1,"label":"API"},{"value":2,"label":"H5"},{"value":3,"label":"管理后台"},{"value":4,"label":"回调"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 选项列表
 * @return_param data.list.value int 选项值
 * @return_param data.list.label string 选项标签
 * @number 1
 */
func CallUpdateSiteDomain(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageOption UpdateSiteDomain")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v14.NewManageOptionClient(conn)
	req := &v14.UpdateSiteDomainReq{
		Id:         gconv.Int64(reqData["id"]),
		SiteCode:   gconv.String(reqData["site_code"]),
		Domain:     gconv.String(reqData["domain"]),
		DomainType: gconv.Int32(reqData["domain_type"]),
		Status:     gconv.Int32(reqData["status"]),
	}

	res, err := client.UpdateSiteDomain(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC ManageOption UpdateSiteDomain failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点游戏管理
 * @title 删除站点域名
 * @description 删除站点域名
 * @method get
 * @url /api/manage/options/domain-type
 * @return {"code":0,"msg":"success","data":{"list":[{"value":1,"label":"API"},{"value":2,"label":"H5"},{"value":3,"label":"管理后台"},{"value":4,"label":"回调"}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 选项列表
 * @return_param data.list.value int 选项值
 * @return_param data.list.label string 选项标签
 * @number 1
 */
func CallDeleteSiteDomain(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageOption DeleteSiteDomain")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v14.NewManageOptionClient(conn)
	req := &v14.DeleteSiteDomainReq{
		Id: gconv.Int64(reqData["id"]),
	}

	res, err := client.DeleteSiteDomain(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC ManageOption DeleteSiteDomain failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/支付管理
 * @title 获取支付列表
 * @description 获取支付列表的接口
 * @method get
 * @url /api/manage/payment/list
 * @param gateway 可选 int 支付网关 1=在线支付 2=银行
 * @param status 可选 int 状态 1=启用 0=禁用
 * @param name 可选 string 名称搜索
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认20
 * @return {"code":0,"msg":"success","data":{"list":[],"total":0}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func CallGetPaymentList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManagePayment GetPaymentList")

	client := v20.NewManagePaymentClient(conn)
	req := &v20.GetPaymentListReq{
		Gateway: r.Get("gateway", 0).Int32(),
		Status:  r.Get("status", -1).Int32(),
		Name:    r.Get("name", "").String(),
		Page:    r.Get("page", 1).Int32(),
		Size:    r.Get("size", 20).Int32(),
	}

	res, err := client.GetPaymentList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Payment GetPaymentList failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/支付管理
 * @title 获取支付详情
 * @description 获取指定支付详情的接口
 * @method get
 * @url /api/manage/payment/detail/{id}
 * @param id 必选 int 支付ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 支付详情数据
 * @number 2
 */
func CallGetPaymentDetail(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManagePayment GetPaymentDetail")

	idStr := r.Get("id").String()
	if idStr == "" {
		util.WriteBadRequest(r, "支付ID不能为空")
		return fmt.Errorf("支付ID不能为空")
	}

	id := r.Get("id").Int32()
	if id <= 0 {
		util.WriteBadRequest(r, "支付ID必须大于0")
		return fmt.Errorf("支付ID必须大于0")
	}

	client := v20.NewManagePaymentClient(conn)
	req := &v20.GetPaymentDetailReq{
		Id: id,
	}

	res, err := client.GetPaymentDetail(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Payment GetPaymentDetail failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/支付管理
 * @title 创建支付
 * @description 创建支付的接口
 * @method post
 * @url /api/manage/payment/create
 * @param name 必选 string 支付名称
 * @param code 必选 string 支付代码
 * @param gateway 必选 int 支付网关 1=在线支付 2=银行
 * @param api_url 可选 string 支付域名
 * @param merchant_no 可选 string 商户号
 * @param md5_key 可选 string MD5密钥
 * @param each_min 可选 float 单笔最低
 * @param each_max 可选 float 单笔最高
 * @param sort 可选 int 排序
 * @param money_list 可选 string 可选金额数组
 * @param is_input 可选 int 是否输入金额 0=否 1=是
 * @param remark 可选 string 描述
 * @param logo 可选 string logo地址
 * @param status 可选 int 状态 0=禁用 1=启用
 * @return {"code":0,"msg":"success","data":{"id":1}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.id int 创建的支付ID
 * @number 3
 */
func CallCreatePayment(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManagePayment CreatePayment")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v20.NewManagePaymentClient(conn)
	req := &v20.CreatePaymentReq{
		Name:       getStringFromMap(reqData, "name"),
		Code:       getStringFromMap(reqData, "code"),
		Gateway:    getInt32FromMap(reqData, "gateway"),
		ApiUrl:     getStringFromMap(reqData, "api_url"),
		MerchantNo: getStringFromMap(reqData, "merchant_no"),
		Md5Key:     getStringFromMap(reqData, "md5_key"),
		EachMin:    gconv.Float64(reqData["each_min"]),
		EachMax:    gconv.Float64(reqData["each_max"]),
		Sort:       getInt32FromMap(reqData, "sort"),
		MoneyList:  getStringFromMap(reqData, "money_list"),
		IsInput:    getInt32FromMap(reqData, "is_input"),
		Remark:     getStringFromMap(reqData, "remark"),
		Logo:       getStringFromMap(reqData, "logo"),
		Status:     getInt32FromMap(reqData, "status"),
	}

	res, err := client.CreatePayment(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Payment CreatePayment failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/支付管理
 * @title 更新支付
 * @description 更新支付信息的接口
 * @method post
 * @url /api/manage/payment/update
 * @param id 必选 int 支付ID
 * @param name 可选 string 支付名称
 * @param code 可选 string 支付代码
 * @param gateway 可选 int 支付网关
 * @param api_url 可选 string 支付域名
 * @param merchant_no 可选 string 商户号
 * @param md5_key 可选 string MD5密钥
 * @param each_min 可选 float 单笔最低
 * @param each_max 可选 float 单笔最高
 * @param sort 可选 int 排序
 * @param money_list 可选 string 可选金额数组
 * @param is_input 可选 int 是否输入金额
 * @param remark 可选 string 描述
 * @param logo 可选 string logo地址
 * @param status 可选 int 状态
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 4
 */
func CallUpdatePayment(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManagePayment UpdatePayment")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v20.NewManagePaymentClient(conn)
	req := &v20.UpdatePaymentReq{
		Id:         getInt32FromMap(reqData, "id"),
		Name:       getStringFromMap(reqData, "name"),
		Code:       getStringFromMap(reqData, "code"),
		Gateway:    getInt32FromMap(reqData, "gateway"),
		ApiUrl:     getStringFromMap(reqData, "api_url"),
		MerchantNo: getStringFromMap(reqData, "merchant_no"),
		Md5Key:     getStringFromMap(reqData, "md5_key"),
		EachMin:    gconv.Float64(reqData["each_min"]),
		EachMax:    gconv.Float64(reqData["each_max"]),
		Sort:       getInt32FromMap(reqData, "sort"),
		MoneyList:  getStringFromMap(reqData, "money_list"),
		IsInput:    getInt32FromMap(reqData, "is_input"),
		Remark:     getStringFromMap(reqData, "remark"),
		Logo:       getStringFromMap(reqData, "logo"),
		Status:     getInt32FromMap(reqData, "status"),
	}

	res, err := client.UpdatePayment(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Payment UpdatePayment failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/支付管理
 * @title 删除支付
 * @description 删除支付的接口
 * @method post
 * @url /api/manage/payment/delete
 * @param id 必选 int 支付ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 5
 */
func CallDeletePayment(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManagePayment DeletePayment")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	id := getInt32FromMap(reqData, "id")
	if id <= 0 {
		util.WriteBadRequest(r, "支付ID必须大于0")
		return fmt.Errorf("支付ID必须大于0")
	}

	client := v20.NewManagePaymentClient(conn)
	req := &v20.DeletePaymentReq{
		Id: id,
	}

	res, err := client.DeletePayment(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Payment DeletePayment failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点支付管理
 * @title 获取站点支付列表
 * @description 获取指定站点的支付列表
 * @method get
 * @url /api/manage/site_payment/list
 * @param site_code 必选 string 站点编码
 * @return {"code":0,"msg":"success","data":{"list":[]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func CallGetSitePaymentList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSitePayment GetSitePaymentList")

	siteCode := r.Get("site_code").String()
	if siteCode == "" {
		util.WriteBadRequest(r, "站点编码不能为空")
		return fmt.Errorf("站点编码不能为空")
	}

	client := v21.NewManageSitePaymentClient(conn)
	req := &v21.GetSitePaymentListReq{
		SiteCode: siteCode,
	}

	res, err := client.GetSitePaymentList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SitePayment GetSitePaymentList failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点支付管理
 * @title 更新站点支付状态
 * @description 更新站点支付的启用/禁用状态
 * @method post
 * @url /api/manage/site_payment/update_status
 * @param id 必选 int 站点支付ID
 * @param status 必选 int 状态 0=禁用 1=启用
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 2
 */
func CallUpdateSitePaymentStatus(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSitePayment UpdateSitePaymentStatus")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v21.NewManageSitePaymentClient(conn)
	req := &v21.UpdateSitePaymentStatusReq{
		Id:     getInt32FromMap(reqData, "id"),
		Status: getInt32FromMap(reqData, "status"),
	}

	res, err := client.UpdateSitePaymentStatus(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SitePayment UpdateSitePaymentStatus failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点支付管理
 * @title 绑定支付到站点
 * @description 将支付方式绑定到指定站点
 * @method post
 * @url /api/manage/site_payment/bind
 * @param site_code 必选 string 站点编码
 * @param payment_id 必选 int 支付ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 3
 */
func CallBindSitePayment(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSitePayment BindSitePayment")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v21.NewManageSitePaymentClient(conn)
	req := &v21.BindSitePaymentReq{
		SiteCode:  getStringFromMap(reqData, "site_code"),
		PaymentId: getInt32FromMap(reqData, "payment_id"),
	}

	res, err := client.BindSitePayment(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SitePayment BindSitePayment failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 总后台/站点支付管理
 * @title 取消绑定支付
 * @description 取消站点与支付方式的绑定
 * @method post
 * @url /api/manage/site_payment/unbind
 * @param id 必选 int 站点支付ID
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 4
 */
func CallUnbindSitePayment(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC ManageSitePayment UnbindSitePayment")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	client := v21.NewManageSitePaymentClient(conn)
	req := &v21.UnbindSitePaymentReq{
		Id: getInt32FromMap(reqData, "id"),
	}

	res, err := client.UnbindSitePayment(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC SitePayment UnbindSitePayment failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}
