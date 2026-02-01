package balance

import (
	"context"
	"encoding/json"

	v6 "jh_gateway/api/backend/balance/v1"
	v17 "jh_gateway/api/frontend/balance/v1"
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
 * @catalog 前台/用户
 * @title OK刷新用户余额
 * @description 刷新用户余额接口
 * @method get
 * @url /api/frontend/balance/refresh-balance
 * @param token 必选 string token
 * @return {"code":0,"msg":"success","data":{"balance":"100.00","balance_frozen":"0.00","points":0}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.balance string 余额
 * @return_param data.balance_frozen string 冻结余额
 * @return_param data.points int 积分
 * @number 109
 */
func CallRefreshBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC RefreshBalance")

	// conn已经是balance服务的连接（由route_dispatcher根据路径/balance自动路由）
	client := v17.NewBalanceClient(conn)
	req := &v17.RefreshBalanceReq{}

	res, err := client.RefreshBalance(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"balance":        res.Balance,
		"balance_frozen": res.BalanceFrozen,
		"points":         res.Points,
	})
	return nil
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
func CallGetBalanceChanges(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetChangeOptions(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetRechargePayments(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetRechargeManuals(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallConfirmPaymentOrder(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetWithdraws(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetWithdrawManuals(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetWithdrawReview(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallDealWithWithdraw(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallQueryUserBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallQueryGameBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallManualUserBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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
func CallGetManualList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
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

/**
 * showdoc
 * @catalog 后台/财务/站点支付
 * @title 获取站点支付列表
 * @description 获取站点支付配置列表
 * @method get
 * @url /api/backend/balance/site-payments
 * @param payment_id 可选 int 支付ID
 * @param status 可选 int 状态
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[],"count":0}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 站点支付列表
 * @return_param data.count int 总数量
 * @remark 获取站点支付配置列表
 * @number 18
 */
func CallGetSitePayments(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetSitePayments")

	// 获取查询参数
	paymentId := r.Get("payment_id", 0).Int32()
	status := r.Get("status", -1).Int32()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetSitePaymentsReq{
		PaymentId: paymentId,
		Status:    status,
		Page:      page,
		Size:      size,
	}

	// 调用 gRPC 服务
	res, err := client.GetSitePayments(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取站点支付列表失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/站点支付
 * @title 更新站点支付
 * @description 更新站点支付配置
 * @method post
 * @url /api/backend/balance/update-site-payment
 * @param id 必选 int 站点支付ID
 * @param status 可选 int 状态
 * @param sort 可选 int 排序
 * @param each_min 可选 float 单笔最低
 * @param each_max 可选 float 单笔最高
 * @param money_list 可选 string 可选金额数组
 * @param is_input 可选 int 是否输入金额
 * @param remark 可选 string 备注
 * @param logo 可选 string logo地址
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"更新成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @remark 更新站点支付配置
 * @number 19
 */
func CallUpdateSitePayment(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance UpdateSitePayment")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt32FromMap(reqData, "id")
	status := getInt32FromMap(reqData, "status")
	sort := getInt32FromMap(reqData, "sort")
	eachMin := getFloat64FromMap(reqData, "each_min")
	eachMax := getFloat64FromMap(reqData, "each_max")
	moneyList := getStringFromMap(reqData, "money_list")
	isInput := getInt32FromMap(reqData, "is_input")
	remark := getStringFromMap(reqData, "remark")
	logo := getStringFromMap(reqData, "logo")

	if id <= 0 {
		util.WriteBadRequest(r, "站点支付ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.UpdateSitePaymentReq{
		Id:        id,
		Status:    status,
		Sort:      sort,
		EachMin:   eachMin,
		EachMax:   eachMax,
		MoneyList: moneyList,
		IsInput:   isInput,
		Remark:    remark,
		Logo:      logo,
	}

	// 调用 gRPC 服务
	res, err := client.UpdateSitePayment(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "更新站点支付失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/站点提现
 * @title 获取站点提现列表
 * @description 获取站点提现配置列表
 * @method get
 * @url /api/backend/balance/site-withdraws
 * @param withdraw_id 可选 int 提现ID
 * @param status 可选 int 状态
 * @param page 可选 int 页码，默认1
 * @param size 可选 int 每页数量，默认50
 * @return {"code":0,"msg":"success","data":{"list":[],"count":0}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.list array 站点提现列表
 * @return_param data.count int 总数量
 * @remark 获取站点提现配置列表
 * @number 20
 */
func CallGetSiteWithdraws(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance GetSiteWithdraws")

	// 获取查询参数
	withdrawId := r.Get("withdraw_id", 0).Int32()
	status := r.Get("status", -1).Int32()
	page := r.Get("page", 1).Int32()
	size := r.Get("size", 50).Int32()

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.GetSiteWithdrawsReq{
		WithdrawId: withdrawId,
		Status:     status,
		Page:       page,
		Size:       size,
	}

	// 调用 gRPC 服务
	res, err := client.GetSiteWithdraws(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "获取站点提现列表失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/财务/站点提现
 * @title 更新站点提现
 * @description 更新站点提现配置
 * @method post
 * @url /api/backend/balance/update-site-withdraw
 * @param id 必选 int 站点提现ID
 * @param status 可选 int 状态
 * @param sort 可选 int 排序
 * @param each_min 可选 float 单笔最低
 * @param each_max 可选 float 单笔最高
 * @param money_list 可选 string 可选金额数组
 * @param is_input 可选 int 是否输入金额
 * @param remark 可选 string 备注
 * @param logo 可选 string logo地址
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"更新成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 数据对象
 * @return_param data.success bool 是否成功
 * @return_param data.message string 响应消息
 * @remark 更新站点提现配置
 * @number 21
 */
func CallUpdateSiteWithdraw(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Balance UpdateSiteWithdraw")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt32FromMap(reqData, "id")
	status := getInt32FromMap(reqData, "status")
	sort := getInt32FromMap(reqData, "sort")
	eachMin := getFloat64FromMap(reqData, "each_min")
	eachMax := getFloat64FromMap(reqData, "each_max")
	moneyList := getStringFromMap(reqData, "money_list")
	isInput := getInt32FromMap(reqData, "is_input")
	remark := getStringFromMap(reqData, "remark")
	logo := getStringFromMap(reqData, "logo")

	if id <= 0 {
		util.WriteBadRequest(r, "站点提现ID不能为空")
		return nil
	}

	// 创建 gRPC 客户端
	client := v6.NewBalanceClient(conn)
	req := &v6.UpdateSiteWithdrawReq{
		Id:        id,
		Status:    status,
		Sort:      sort,
		EachMin:   eachMin,
		EachMax:   eachMax,
		MoneyList: moneyList,
		IsInput:   isInput,
		Remark:    remark,
		Logo:      logo,
	}

	// 调用 gRPC 服务
	res, err := client.UpdateSiteWithdraw(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "更新站点提现失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/资金
 * @title OK获取支付渠道列表
 * @description 获取可用的支付渠道列表
 * @method get
 * @url /api/frontend/balance/payment-list
 * @header Authorization 必选 string Bearer {token}
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"site_id":1,"payment_id":1,"name":"在线支付","code":"online","gateway":1,"each_min":10,"each_max":50000,"sort":100,"money_list":["100","500","1000","5000"],"is_input":1,"remark":"支持多种支付方式","logo":"https://example.com/logo.png","status":1}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.list array 支付渠道列表
 * @return_param data.list.id int 渠道ID
 * @return_param data.list.site_id int 站点ID
 * @return_param data.list.payment_id int 总站支付ID
 * @return_param data.list.name string 渠道名称
 * @return_param data.list.code string 渠道代码
 * @return_param data.list.gateway int 支付网关：1=在线支付 2=银行
 * @return_param data.list.each_min float 单笔最低金额
 * @return_param data.list.each_max float 单笔最高金额
 * @return_param data.list.sort int 排序（从大到小）
 * @return_param data.list.money_list array 可选金额数组
 * @return_param data.list.is_input int 是否支持输入金额：0=不支持 1=支持
 * @return_param data.list.remark string 描述
 * @return_param data.list.logo string logo地址
 * @return_param data.list.status int 状态
 * @remark 只返回status=1的可用渠道，按sort从大到小排序
 * @number 200
 */
func CallGetPaymentList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetPaymentList")

	client := v17.NewBalanceClient(conn)
	req := &v17.GetPaymentListReq{}

	res, err := client.GetPaymentList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取支付渠道列表失败")
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/资金
 * @title OK获取提现渠道列表
 * @description 获取可用的提现渠道列表
 * @method get
 * @url /api/frontend/balance/withdraw-list
 * @header Authorization 必选 string Bearer {token}
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"site_id":1,"withdraw_id":1,"name":"银行卡提现","code":"bank","gateway":2,"each_min":100,"each_max":50000,"sort":100,"money_list":["100","500","1000","5000"],"is_input":1,"remark":"支持多家银行","logo":"https://example.com/logo.png","status":1}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.list array 提现渠道列表
 * @return_param data.list.id int 渠道ID
 * @return_param data.list.site_id int 站点ID
 * @return_param data.list.withdraw_id int 总站提现ID
 * @return_param data.list.name string 渠道名称
 * @return_param data.list.code string 渠道代码
 * @return_param data.list.gateway int 提现网关：1=在线支付 2=银行
 * @return_param data.list.each_min float 单笔最低金额
 * @return_param data.list.each_max float 单笔最高金额
 * @return_param data.list.sort int 排序（从大到小）
 * @return_param data.list.money_list array 可选金额数组
 * @return_param data.list.is_input int 是否支持输入金额：0=不支持 1=支持
 * @return_param data.list.remark string 描述
 * @return_param data.list.logo string logo地址
 * @return_param data.list.status int 状态
 * @remark 只返回status=1的可用渠道，按sort从大到小排序
 * @number 201
 */
func CallGetWithdrawList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetWithdrawList")

	client := v17.NewBalanceClient(conn)
	req := &v17.GetWithdrawListReq{}

	res, err := client.GetWithdrawList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, "获取提现渠道列表失败")
		return nil
	}

	// 使用统一的protobuf JSON序列化函数
	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/银行卡
 * @title 添加银行卡
 * @description 添加银行卡接口
 * @method post
 * @url /api/frontend/balance/add-bank-card
 * @param type 必选 int 类型 1=GCASH 2=Maya
 * @param card_account 必选 string 账户名
 * @param card_no 必选 string 卡号
 * @param is_default 可选 int 是否默认 0=否 1=是
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"添加成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 消息
 * @number 110
 */
func CallAddBankCard(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC AddBankCard")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	cardType := getInt32FromMap(reqData, "type")
	cardAccount := getStringFromMap(reqData, "card_account")
	cardNo := getStringFromMap(reqData, "card_no")
	isDefault := getInt32FromMapWithDefault(reqData, "is_default", 0)

	client := v17.NewBalanceClient(conn)
	req := &v17.AddBankCardReq{
		Type:        cardType,
		CardAccount: cardAccount,
		CardNo:      cardNo,
		IsDefault:   isDefault,
	}

	res, err := client.AddBankCard(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"success": res.Success,
		"message": res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/银行卡
 * @title 查询银行卡列表
 * @description 查询用户的银行卡列表
 * @method get
 * @url /api/frontend/balance/bank-card-list
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"type":1,"card_account":"John Doe","card_no":"09123456789","is_default":1,"created_at":"2024-01-01 10:00:00"}]}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.list array 银行卡列表
 * @number 111
 */
func CallGetBankCardList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetBankCardList")

	client := v17.NewBalanceClient(conn)
	req := &v17.GetBankCardListReq{}

	res, err := client.GetBankCardList(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"list": res.List,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/银行卡
 * @title 删除银行卡
 * @description 删除银行卡接口
 * @method post
 * @url /api/frontend/balance/delete-bank-card
 * @param id 必选 int 银行卡ID
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"删除成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 消息
 * @number 112
 */
func CallDeleteBankCard(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC DeleteBankCard")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt32FromMap(reqData, "id")

	client := v17.NewBalanceClient(conn)
	req := &v17.DeleteBankCardReq{
		Id: id,
	}

	res, err := client.DeleteBankCard(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"success": res.Success,
		"message": res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/银行卡
 * @title 设置默认银行卡
 * @description 设置默认银行卡接口
 * @method post
 * @url /api/frontend/balance/set-default-bank-card
 * @param id 必选 int 银行卡ID
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"设置成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 消息
 * @number 113
 */
func CallSetDefaultBankCard(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC SetDefaultBankCard")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	id := getInt32FromMap(reqData, "id")

	client := v17.NewBalanceClient(conn)
	req := &v17.SetDefaultBankCardReq{
		Id: id,
	}

	res, err := client.SetDefaultBankCard(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"success": res.Success,
		"message": res.Message,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/资金
 * @title 获取充值 nonce
 * @description 获取充值防重放攻击的随机数
 * @method get
 * @url /api/frontend/balance/deposit-nonce
 * @header Authorization 必选 string Bearer {token}
 * @return {"code":0,"msg":"success","data":{"nonce":"550e8400-e29b-41d4-a716-446655440000","timestamp":1640000000,"expires_in":60}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.nonce string 随机数（UUID格式）
 * @return_param data.timestamp int 时间戳
 * @return_param data.expires_in int 过期时间（秒）
 * @remark 用于防止充值请求的重复提交和重放攻击，nonce有效期60秒
 * @number 114
 */
func CallGetDepositNonce(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetDepositNonce")

	client := v17.NewBalanceClient(conn)
	req := &v17.GetDepositNonceReq{}

	res, err := client.GetDepositNonce(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	util.LogWithTrace(ctx, "info", "GetDepositNonce response: nonce=%s, timestamp=%d, expires_in=%d", res.Nonce, res.Timestamp, res.ExpiresIn)

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"nonce":      res.Nonce,
		"timestamp":  res.Timestamp,
		"expires_in": res.ExpiresIn,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/资金
 * @title 获取提现 nonce
 * @description 获取提现防重放攻击的随机数
 * @method get
 * @url /api/frontend/balance/withdraw-nonce
 * @header Authorization 必选 string Bearer {token}
 * @return {"code":0,"msg":"success","data":{"nonce":"550e8400-e29b-41d4-a716-446655440000","timestamp":1640000000,"expires_in":60}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.nonce string 随机数（UUID格式）
 * @return_param data.timestamp int 时间戳
 * @return_param data.expires_in int 过期时间（秒）
 * @remark 用于防止提现请求的重复提交和重放攻击，nonce有效期60秒
 * @number 115
 */
func CallGetWithdrawNonce(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetWithdrawNonce")

	client := v17.NewBalanceClient(conn)
	req := &v17.GetWithdrawNonceReq{}

	res, err := client.GetWithdrawNonce(ctx, req)
	if err != nil {
		util.WriteInternalError(r, err.Error())
		return nil
	}

	util.LogWithTrace(ctx, "info", "GetWithdrawNonce response: nonce=%s, timestamp=%d, expires_in=%d", res.Nonce, res.Timestamp, res.ExpiresIn)

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"nonce":      res.Nonce,
		"timestamp":  res.Timestamp,
		"expires_in": res.ExpiresIn,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/资金
 * @title 创建充值订单
 * @description 创建充值订单接口
 * @method post
 * @url /api/frontend/balance/deposit-order
 * @header Authorization 必选 string Bearer {token}
 * @param payment_id 必选 int 支付方式ID
 * @param money 必选 float 充值金额
 * @param order_type 可选 string 订单类型(deposit=充值,withdraw=提现),默认deposit
 * @param user_card_no 可选 string 用户银行卡号
 * @param user_card_name 可选 string 用户银行卡姓名
 * @param nonce 必选 string 防重放随机数
 * @param bank_code 可选 string 银行代码
 * @param mobile 可选 int 是否手机端(1=是,0=否)
 * @return {"code":0,"msg":"success","data":{"order_sn":"ZF0131220145123456","sys_trade_no":"sys_7255ac6c-a38f-40f5-bf2f-f9f19745d877","channel_card_no":"09678682672","channel_name":"KBZ","image_url":"https://img.okszckoo.com/upload/images/202502/c18cd50f-1025-48dd-af55-c5d49a819cc5.jpg","result":"ok"}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.order_sn string 系统订单号
 * @return_param data.sys_trade_no string 平台交易流水号
 * @return_param data.channel_card_no string 收款卡号
 * @return_param data.channel_name string 渠道名称
 * @return_param data.image_url string 支付引导图
 * @return_param data.result string 返回信息
 * @remark 创建充值订单，需要先调用deposit-nonce获取nonce
 * @number 116
 */
func CallCreateDepositOrder(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC CreateDepositOrder")

	client := v17.NewBalanceClient(conn)

	var req v17.CreateDepositOrderReq
	if err := r.Parse(&req); err != nil {
		util.LogWithTrace(ctx, "error", "参数解析失败: %v", err)
		util.WriteInternalError(r, "参数解析失败")
		return nil
	}

	util.LogWithTrace(ctx, "info", "CreateDepositOrder request: payment_id=%d, money=%.2f, nonce=%s",
		req.PaymentId, req.Money, req.Nonce)

	res, err := client.CreateDepositOrder(ctx, &req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "CreateDepositOrder failed: %v", err)
		util.WriteInternalError(r, err.Error())
		return nil
	}

	util.LogWithTrace(ctx, "info", "CreateDepositOrder response: order_sn=%s, sys_trade_no=%s",
		res.OrderSn, res.SysTradeNo)

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"order_sn":        res.OrderSn,
		"sys_trade_no":    res.SysTradeNo,
		"channel_card_no": res.ChannelCardNo,
		"channel_name":    res.ChannelName,
		"image_url":       res.ImageUrl,
		"result":          res.Result,
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/资金
 * @title 获取账户明细
 * @description 获取账户明细列表
 * @method get
 * @url /api/frontend/balance/balance-log
 * @header Authorization 必选 string Bearer {token}
 * @param start 必选 string 开始时间(格式:2024-01-01 00:00:00)
 * @param end 必选 string 结束时间(格式:2024-01-01 23:59:59)
 * @param type 可选 int 类型(0=全部,1=充值,2=提款,3=红利,4=返水,5=转账)
 * @param status 可选 int 状态(0=全部,1=待处理,2=成功,3=失败)
 * @param page 可选 int 页码(默认1)
 * @param size 可选 int 每页数量(默认20)
 * @return {"code":0,"msg":"success","data":{"total_recharge":1000.00,"total_withdraw":200.00,"total_bonus":50.00,"total_rebate":10.50,"list":[{"channel":"KBZ","change_type":1,"trade_no":"ZF0131220145123456","money":100.00,"remark":"充值","type":1,"symbol":"+","status":2,"time":"2024-01-01 12:00:00"}],"count":10}}
 * @return_param code int 状态码
 * @return_param msg string 提示说明
 * @return_param data object 主要数据
 * @return_param data.total_recharge float 充值总计
 * @return_param data.total_withdraw float 提现总计
 * @return_param data.total_bonus float 红利总计
 * @return_param data.total_rebate float 返水总计
 * @return_param data.list array 明细列表
 * @return_param data.list.channel string 渠道名称
 * @return_param data.list.change_type int 交易类型(1=转入,2=转出)
 * @return_param data.list.trade_no string 订单号
 * @return_param data.list.money float 金额
 * @return_param data.list.remark string 备注
 * @return_param data.list.type int 类型(1=充值,2=提款,3=红利,4=返水,5=转账)
 * @return_param data.list.symbol string 符号(+或-)
 * @return_param data.list.status int 状态(1=待处理,2=成功,3=失败)
 * @return_param data.list.time string 时间
 * @return_param data.count int 总数
 * @remark 查询用户账户明细记录
 * @number 117
 */
func CallGetBalanceLog(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC GetBalanceLog")

	client := v17.NewBalanceClient(conn)

	// 从 query 参数中获取数据
	req := v17.GetBalanceLogReq{
		Start:  r.Get("start").String(),
		End:    r.Get("end").String(),
		Type:   r.Get("type").Int32(),
		Status: r.Get("status").Int32(),
		Page:   r.Get("page").Int32(),
		Size:   r.Get("size").Int32(),
	}

	util.LogWithTrace(ctx, "info", "GetBalanceLog request: start=%s, end=%s, type=%d, status=%d, page=%d, size=%d",
		req.Start, req.End, req.Type, req.Status, req.Page, req.Size)

	res, err := client.GetBalanceLog(ctx, &req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "GetBalanceLog failed: %v", err)
		util.WriteInternalError(r, err.Error())
		return nil
	}

	util.LogWithTrace(ctx, "info", "GetBalanceLog response: count=%d", res.Count)

	// 转换列表数据
	list := make([]map[string]interface{}, 0, len(res.List))
	for _, item := range res.List {
		list = append(list, map[string]interface{}{
			"channel":     item.Channel,
			"change_type": item.ChangeType,
			"trade_no":    item.TradeNo,
			"money":       item.Money,
			"remark":      item.Remark,
			"type":        item.Type,
			"symbol":      item.Symbol,
			"status":      item.Status,
			"time":        item.Time,
		})
	}

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"total_recharge": res.TotalRecharge,
		"total_withdraw": res.TotalWithdraw,
		"total_bonus":    res.TotalBonus,
		"total_rebate":   res.TotalRebate,
		"list":           list,
		"count":          res.Count,
	})
	return nil
}

/**
 * @catalog 前台接口/余额模块
 * @title 充值异步回调
 * @description 充值异步回调接口
 * @method post
 * @url /api/frontend/balance/deposit-callback
 * @param trade_status 必选 string 交易状态(success:成功,failed:失败,wait:处理中,cancel:取消)
 * @param order_sn 必选 string 订单号
 * @param actual_amount 必选 number 实际支付金额
 * @param sign 可选 string 签名
 * @return {"code":200,"msg":"success","data":{"success":true,"message":"支付成功"}}
 * @return_param code int 状态码
 * @return_param msg string 提示信息
 * @return_param data object 返回数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 消息
 * @remark 此接口用于第三方支付平台异步通知充值结果
 * @number 99
 */
func CallDepositCallback(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC DepositCallback")

	client := v17.NewBalanceClient(conn)

	var req v17.DepositCallbackReq
	if err := r.Parse(&req); err != nil {
		util.LogWithTrace(ctx, "error", "参数解析失败: %v", err)
		util.WriteInternalError(r, "参数解析失败")
		return nil
	}

	util.LogWithTrace(ctx, "info", "DepositCallback request: trade_status=%s, order_sn=%s, actual_amount=%.2f",
		req.TradeStatus, req.OrderSn, req.ActualAmount)

	res, err := client.DepositCallback(ctx, &req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "DepositCallback failed: %v", err)
		util.WriteInternalError(r, err.Error())
		return nil
	}

	util.LogWithTrace(ctx, "info", "DepositCallback response: success=%v, message=%s", res.Success, res.Message)

	// 返回成功响应
	util.WriteSuccess(r, map[string]interface{}{
		"success": res.Success,
		"message": res.Message,
	})
	return nil
}

// CallWithdraw 申请提现
func CallWithdraw(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Withdraw")

	// 使用中间件解析的请求数据
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return nil
	}

	// 获取参数
	money := getFloat64FromMap(reqData, "money")
	withdrawId := getInt32FromMap(reqData, "withdraw_id")
	nonce := getStringFromMap(reqData, "nonce")

	util.LogWithTrace(ctx, "info", "解析参数 - money: %.2f, withdraw_id: %d, nonce: %s, reqData: %+v", money, withdrawId, nonce, reqData)

	if money <= 0 {
		util.LogWithTrace(ctx, "error", "提现金额无效: %.2f", money)
		util.WriteBadRequest(r, "提现金额必须大于0")
		return nil
	}

	if withdrawId <= 0 {
		util.WriteBadRequest(r, "请选择提现渠道")
		return nil
	}

	// 创建 gRPC 客户端
	client := v17.NewBalanceClient(conn)
	req := &v17.WithdrawReq{
		Money:      money,
		WithdrawId: withdrawId,
		Nonce:      nonce,
	}

	util.LogWithTrace(ctx, "info", "gRPC请求参数 - Money: %.2f, WithdrawId: %d, Nonce: %s", req.Money, req.WithdrawId, req.Nonce)

	// 调用 gRPC 服务
	resp, err := client.Withdraw(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC调用失败: %v", err)
		util.WriteInternalError(r, "申请提现失败，请稍后重试")
		return nil
	}

	// 使用统一的protobuf响应序列化
	return writeProtobufResponse(ctx, r, resp)
}
