package util

import (
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
	"google.golang.org/grpc/status"
)

// Response 统一响应结构
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// Error codes
const (
	CodeSuccess            = 0
	CodeBadRequest         = 400
	CodeUnauthorized       = 401
	CodeForbidden          = 403
	CodeNotFound           = 404
	CodeTooManyRequests    = 429
	CodeInternalError      = 500
	CodeServiceUnavailable = 503
)

// ErrorMessages 错误消息映射
var ErrorMessages = map[int]string{
	CodeSuccess:            "success",
	CodeBadRequest:         "bad request",
	CodeUnauthorized:       "unauthorized",
	CodeForbidden:          "forbidden",
	CodeNotFound:           "not found",
	CodeTooManyRequests:    "too many requests",
	CodeInternalError:      "internal server error",
	CodeServiceUnavailable: "service unavailable",
}

// ExtractGRPCErrorMessage 从gRPC错误中提取纯净的错误消息
// 将 "rpc error: code = Unknown desc = 已经绑定过密码请勿重新修改" 转换为 "已经绑定过密码请勿重新修改"
func ExtractGRPCErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	// 尝试使用grpc status提取错误消息
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}

	// 如果不是标准gRPC错误，尝试手动解析
	errStr := err.Error()

	// 处理 "rpc error: code = XXX desc = YYY" 格式
	if strings.Contains(errStr, "desc = ") {
		parts := strings.SplitN(errStr, "desc = ", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}

	return errStr
}

// WriteSuccess 返回成功响应
func WriteSuccess(r *ghttp.Request, data interface{}) {
	r.Response.WriteJsonExit(Response{
		Code: CodeSuccess,
		Msg:  ErrorMessages[CodeSuccess],
		Data: data,
	})
}

// WriteError 返回错误响应
func WriteError(r *ghttp.Request, code int, msg string) {
	if msg == "" {
		msg = ErrorMessages[code]
	}
	msg = NormalizeMessageWithPath(msg, r.URL.Path)
	// 不设置HTTP状态码，始终返回200，错误信息在JSON中体现
	r.Response.WriteJsonExit(Response{
		Code: code,
		Msg:  msg,
	})
}

// WriteBadRequest 400 错误
func WriteBadRequest(r *ghttp.Request, msg string) {
	WriteError(r, CodeBadRequest, msg)
}

// WriteUnauthorized 401 错误
func WriteUnauthorized(r *ghttp.Request, msg string) {
	WriteError(r, CodeUnauthorized, msg)
}

// WriteForbidden 403 错误
func WriteForbidden(r *ghttp.Request, msg string) {
	WriteError(r, CodeForbidden, msg)
}

// WriteNotFound 404 错误
func WriteNotFound(r *ghttp.Request, msg string) {
	WriteError(r, CodeNotFound, msg)
}

// WriteTooManyRequests 429 错误
func WriteTooManyRequests(r *ghttp.Request, msg string) {
	WriteError(r, CodeTooManyRequests, msg)
}

// WriteInternalError 500 错误
func WriteInternalError(r *ghttp.Request, msg string) {
	// 清理gRPC错误前缀
	cleanMsg := cleanGRPCErrorMessage(msg)
	WriteError(r, CodeInternalError, cleanMsg)
}

// NormalizeMessage 清理gRPC错误消息中的前缀（不做翻译）
func NormalizeMessage(msg string) string {
	return NormalizeMessageWithPath(msg, "")
}

// NormalizeMessageWithPath 根据路径决定是否翻译
func NormalizeMessageWithPath(msg string, path string) string {
	msg = cleanGRPCErrorMessage(msg)
	if shouldTranslate(path) {
		return translateMessage(msg)
	}
	return msg
}

// 仅对前台路径翻译
func shouldTranslate(path string) bool {
	return strings.HasPrefix(path, "/front/") || strings.HasPrefix(path, "/api/frontend")
}

// cleanGRPCErrorMessage 清理gRPC错误消息中的前缀
func cleanGRPCErrorMessage(msg string) string {
	// 处理 "rpc error: code = XXX desc = YYY" 格式
	if strings.Contains(msg, "desc = ") {
		parts := strings.SplitN(msg, "desc = ", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return msg
}

// translateMessage 将常见中文提示映射为英文（最少改动，未命中则原样返回）
func translateMessage(msg string) string {
	if msg == "" {
		return msg
	}

	originalMsg := msg

	// 去除常见标点再尝试匹配
	trimmed := strings.TrimSpace(msg)
	trimmed = strings.TrimSuffix(trimmed, "。")

	if strings.HasPrefix(trimmed, "余额不足，当前余额:") {
		suffix := strings.TrimPrefix(trimmed, "余额不足，当前余额:")
		suffix = strings.ReplaceAll(suffix, "需要", "required")
		return "Insufficient balance, current:" + suffix
	}
	if strings.HasPrefix(trimmed, "支付失败，通知状态：") {
		suffix := strings.TrimPrefix(trimmed, "支付失败，通知状态：")
		return "Payment failed, notify status: " + strings.TrimSpace(suffix)
	}

	suffixMapping := map[string]string{
		"游戏关闭维护": "Game is under maintenance",
	}
	for suffix, translated := range suffixMapping {
		if strings.HasSuffix(trimmed, suffix) || strings.HasSuffix(msg, suffix) {
			return translated
		}
	}

	prefixMapping := map[string]string{
		"Balance类型不支持":            "Unsupported balance type",
		"Balance解析失败":             "Failed to parse balance",
		"HTTP错误":                  "HTTP error",
		"TransferIn游戏转账失败。错误代码":   "TransferIn failed. Error code",
		"body字段类型不正确":             "Invalid body field type",
		"data.amount 类型不正确":       "Invalid data.amount type",
		"不支持的支付平台":                "Unsupported payment platform",
		"不支持的游戏平台":                "Unsupported game platform",
		"从旧游戏转出失败":                "Failed to transfer out from legacy game",
		"创建旧游戏平台实例失败":             "Failed to create legacy game platform instance",
		"创建游戏平台实例失败":              "Failed to create game platform instance",
		"创建订单失败":                  "Failed to create order",
		"创建转出订单失败":                "Failed to create transfer-out order",
		"同步游戏到数据库失败":              "Failed to sync games to database",
		"开始时间格式错误":                "Invalid start time format",
		"扣款失败":                    "Debit failed",
		"批量插入注单失败":                "Batch insert bet records failed",
		"数据库连接不存在":                "Database connection not found",
		"更新转账状态失败":                "Failed to update transfer status",
		"更新余额失败":                  "Failed to update balance",
		"查询余额失败":                  "Failed to query balance",
		"查询历史投注失败":                "Failed to query bet history",
		"查询旧游戏配置失败":               "Failed to query legacy game config",
		"查询游戏信息失败":                "Failed to query game info",
		"查询游戏种类失败":                "Failed to query game categories",
		"查询游戏配置失败":                "Failed to query game config",
		"查询订单失败":                  "Failed to query order",
		"查询流水要求列表失败":              "Failed to query flow requirements list",
		"查询流水要求总数失败":              "Failed to query flow requirements count",
		"注册API请求失败":               "Register API request failed",
		"注册会员失败":                  "Failed to register member",
		"注册失败":                    "Registration failed",
		"游戏API余额请求失败":             "Game API balance request failed",
		"游戏API请求失败":               "Game API request failed",
		"游戏API返回格式错误，body字段类型不正确": "Game API response format error: body field type invalid",
		"游戏API返回格式错误，data字段类型不正确": "Game API response format error: data field type invalid",
		"游戏API返回格式错误，无法获取游戏链接":    "Game API response format error: cannot get game link",
		"游戏API返回格式错误，缺少body字段":    "Game API response format error: missing body field",
		"游戏API返回格式错误，缺少data字段":    "Game API response format error: missing data field",
		"游戏不存在: gameId=":          "Game not found: gameId=",
		"游戏转出失败。错误代码":             "Game transfer out failed. Error code",
		"生成签名失败":                  "Failed to generate signature",
		"结束时间格式错误":                "Invalid end time format",
		"网络请求失败，请稍后再试。HTTP代码":     "Network request failed, please try again later. HTTP code",
		"获取最后游戏记录失败":              "Failed to get last game record",
		"获取游戏列表失败":                "Failed to get game list",
		"获取游戏链接失败":                "Failed to get game link",
		"获取用户余额失败":                "Failed to get user balance",
		"获取记录总数失败":                "Failed to get total count",
		"获取运营商列表失败":               "Failed to get operator list",
		"获取运营商失败":                 "Failed to get operator",
		"解析API响应失败":               "Failed to parse API response",
		"解析JSON失败":                "Failed to parse JSON",
		"解析余额响应失败":                "Failed to parse balance response",
		"解析响应失败":                  "Failed to parse response",
		"解析注册响应失败":                "Failed to parse register response",
		"解析游戏API响应失败":             "Failed to parse game API response",
		"请求API失败":                 "API request failed",
		"请求支付网关失败":                "Failed to request payment gateway",
		"调用balance服务失败":           "Failed to call balance service",
		"转入游戏失败":                  "Failed to transfer into game",
		"转出失败":                    "Transfer out failed",
		"转账失败:":                   "Transfer failed:",
		"连接balance服务失败":           "Failed to connect to balance service",
		"支付匹配失败":                  "Payment match failed",
		"记录游戏日志失败":                "Failed to record game log",
		"记录账变日志失败":                "Failed to record balance log",
		"记录资金日志失败":                "Failed to record fund log",
		"支付资金处理失败":                "Payment processing failed",
		"未知的交易类型":                 "Unknown trade type",
		"订单不存在:":                  "Order not found:",
		"生成验证码失败":                 "Failed to generate captcha",
		"获取站点CODE失败":              "Failed to get site code",
		"获取站点配置失败":                "Failed to get site config",
		"获取站点信息失败":                "Failed to get site info",
		"查询公告列表失败":                "Failed to query notice list",
		"查询广告列表失败":                "Failed to query ad list",
		"缺少认证token，请先登录":          "First login and invalid token",
		"token无效或已过期，请重新登录":       "Please login and invalid token",
	}
	for prefix, translated := range prefixMapping {
		if strings.HasPrefix(trimmed, prefix) || strings.HasPrefix(msg, prefix) {
			return translated
		}
	}

	mapping := map[string]string{
		"success": "success",
		"申请提现成功":  "Withdraw success",
		"您还有未完成的流水要求，暂不可提现": "You have unfinished turnover requirements and cannot withdraw",
		"成功":                  "success",
		"失败":                  "failed",
		"验证码错误":               "Incorrect captcha",
		"验证码已过期，请重新获取":        "Captcha expired, please refresh",
		"请输入验证码":              "Please enter captcha",
		"请输入验证码,":             "Please enter captcha",
		"验证码验证失败":             "Captcha verification failed",
		"用户名必填":               "Username is required",
		"密码必填":                "Password is required",
		"时间戳必填":               "Timestamp is required",
		"用户名已经被注册":            "Username already exists",
		"手机账号已经被注册":           "Mobile number already exists",
		"用户名长度需为6-16位":        "Username length must be 6-16 characters",
		"密码长度需为6-20位":         "Password length must be 6-20 characters",
		"用户名仅允许字母和数字":         "Username can only contain letters and numbers",
		"注册失败":                "Registration failed",
		"登录失败":                "Login failed",
		"IG_JILI平台暂不支持游戏资源同步": "IG_JILI platform does not support game resource sync",
		"resData 类型错误，不是对象":   "Invalid resData type",
		"不支持该货币":              "Unsupported currency",
		"创建游戏平台实例失败":          "Failed to create game platform instance",
		"功能待实现":               "Feature not implemented",
		"响应数据格式错误":            "Invalid response data format",
		"回收成功":                "Recycled successfully",
		"开始时间和结束时间不能为空":       "Start and end time are required",
		"无法获取游戏链接":            "Unable to get game link",
		"无需回收，您当前未在游戏中":       "No recycle needed, you are not in game",
		"未查询到玩家信息(Data不存在)":   "Player info not found (Data missing)",
		"未查询到玩家信息(Data为空)":    "Player info not found (Data empty)",
		"未知错误":                "Unknown error",
		"未经授权的访问":             "Unauthorized access",
		"查询成功":                "Query successful",
		"查询游戏余额失败":            "Failed to query game balance",
		"查询游戏状态失败":            "Failed to query game status",
		"查询游戏配置失败":            "Failed to query game config",
		"游戏API返回格式错误，body字段类型不正确": "Game API response format error: body field type invalid",
		"游戏API返回格式错误，data字段类型不正确": "Game API response format error: data field type invalid",
		"游戏API返回格式错误，无法获取游戏链接":    "Game API response format error: cannot get game link",
		"游戏API返回格式错误，缺少body字段":    "Game API response format error: missing body field",
		"游戏API返回格式错误，缺少data字段":    "Game API response format error: missing data field",
		"游戏ID无效": "Invalid game ID",
		"游戏不存在":  "Game not found",
		"玩家信息格式错误(Data[0]不是对象)": "Player info format error (Data[0] is not object)",
		"缺少body字段":                      "Missing body field",
		"缺少字段 data.amount":              "Missing field data.amount",
		"获取用户信息失败":                      "Failed to get user info",
		"请求参数不能为空":                      "Request parameters cannot be empty",
		"请求无效":                          "Invalid request",
		"请求重复":                          "Duplicate request",
		"转账失败":                          "Transfer failed",
		"支付匹配失败":                        "Payment match failed",
		"支付平台类型错误":                      "Payment platform type error",
		"更新余额失败":                        "Failed to update balance",
		"查询失败":                          "Query failed",
		"输出失败":                          "Output failed",
		"连接余额服务失败":                      "Failed to connect to balance service",
		"钱包类型不支持":                       "Unsupported wallet type",
		"错误代码:%d, 消息:%s":                "Error code:%d, message:%s",
		"验证失败":                          "Verification failed",
		"站点ID无效":                        "Invalid site ID",
		"用户ID无效":                        "Invalid user ID",
		"用户名不能为空":                       "Username is required",
		"用户余额已存在":                       "User balance already exists",
		"用户余额初始化成功":                     "User balance initialized successfully",
		"扣款请求已提交，等待处理":                  "Debit request submitted, pending processing",
		"更新转账状态成功":                      "Transfer status updated successfully",
		"处理失败订单成功":                      "Failed order processed successfully",
		"转出请求已提交，等待处理":                  "Transfer-out request submitted, pending processing",
		"未获取用户ID":                       "User ID not found",
		"未获取站点ID":                       "Site ID not found",
		"类型无效，必须是1(GCASH)或2(Maya)":      "Invalid type, must be 1 (GCASH) or 2 (Maya)",
		"账户名不能为空":                       "Account name is required",
		"卡号不能为空":                        "Card number is required",
		"查询银行卡失败":                       "Failed to query bank card",
		"更新默认状态失败":                      "Failed to update default status",
		"添加银行卡失败":                       "Failed to add bank card",
		"添加成功":                          "Added successfully",
		"银行卡ID无效":                       "Invalid bank card ID",
		"删除银行卡失败":                       "Failed to delete bank card",
		"银行卡不存在或无权删除":                   "Bank card not found or no permission to delete",
		"删除成功":                          "Deleted successfully",
		"银行卡不存在或无权操作":                   "Bank card not found or no permission to operate",
		"设置默认银行卡失败":                     "Failed to set default bank card",
		"设置成功":                          "Set successfully",
		"支付失败，通知状态：":                    "Payment failed, notify status: ",
		"查询订单失败":                        "Failed to query order",
		"订单不存在":                         "Order not found",
		"订单已完成":                         "Order already completed",
		"订单已失败":                         "Order already failed",
		"支付成功":                          "Payment successful",
		"未登录":                           "Not logged in",
		"用户余额记录不存在":                     "User balance record not found",
		"用户冻结金额不足":                      "Insufficient frozen balance",
		"查询支付渠道失败":                      "Failed to query payment channel",
		"查询提现渠道失败":                      "Failed to query withdrawal channel",
		"查询用户余额失败":                      "Failed to query user balance",
		"查询银行卡列表失败":                     "Failed to query bank card list",
		"获取余额失败":                        "Failed to get balance",
		"用户名或密码错误":                      "Invalid username or password",
		"refresh_token必填":               "Refresh token is required",
		"refresh token无效或已过期":           "Refresh token is invalid or expired",
		"refresh token格式错误":             "Refresh token format is invalid",
		"token类型错误":                     "Token type is invalid",
		"refresh token已被使用，请使用最新的token": "Refresh token has been used, please use the latest token",
		"refresh token中缺少用户信息":          "Refresh token missing user info",
		"refresh token中缺少站点信息":          "Refresh token missing site info",
		"用户不存在或已禁用":                     "User not found or disabled",
		"刷新token失败":                     "Failed to refresh token",
		"注册服务暂时不可用，请稍后重试":               "Registration service unavailable, please try again later",
	}

	if v, ok := mapping[trimmed]; ok {
		return v
	}
	if v, ok := mapping[msg]; ok {
		return v
	}

	// 兜底：仍包含中文则统一返回英文并记录日志
	for _, r := range originalMsg {
		if r >= '\u4e00' && r <= '\u9fff' {
			g.Log().Warningf(gctx.New(), "translate fallback: %s", originalMsg)
			return "Operation failed"
		}
	}
	return msg
}

// WriteServiceUnavailable 503 错误
func WriteServiceUnavailable(r *ghttp.Request, msg string) {
	WriteError(r, CodeServiceUnavailable, msg)
}
