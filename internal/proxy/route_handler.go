package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"google.golang.org/grpc"
)

// RouteHandler 定义路由处理函数类型
type RouteHandler func(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error

// RouteKey 路由键，包含路径后缀和HTTP方法
type RouteKey struct {
	PathSuffix string
	Method     string
}

// RouteRegistry 路由注册表
type RouteRegistry struct {
	routes map[RouteKey]RouteHandler
}

// NewRouteRegistry 创建新的路由注册表
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		routes: make(map[RouteKey]RouteHandler),
	}
}

// Register 注册路由
func (rr *RouteRegistry) Register(pathSuffix, method string, handler RouteHandler) {
	key := RouteKey{
		PathSuffix: pathSuffix,
		Method:     method,
	}
	rr.routes[key] = handler
}

// Handle 处理请求
func (rr *RouteRegistry) Handle(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	path := r.URL.Path
	method := r.Method

	// 遍历所有注册的路由，找到匹配的处理函数
	for key, handler := range rr.routes {
		if strings.HasSuffix(path, key.PathSuffix) && method == key.Method {
			return handler(ctx, conn, r)
		}
	}

	return fmt.Errorf("unsupported path: %s %s", method, path)
}

// 全局路由注册表
var (
	appRoutes     *RouteRegistry
	balanceRoutes *RouteRegistry
	gameRoutes    *RouteRegistry
)

// 初始化路由注册表
func init() {
	initAppRoutes()
	initBalanceRoutes()
	initGameRoutes()
}

// initAppRoutes 初始化应用路由
func initAppRoutes() {
	appRoutes = NewRouteRegistry()

	// 管理员相关接口
	appRoutes.Register("/login", "POST", callAdminLogin)
	appRoutes.Register("/refresh-token", "GET", callAdminRefreshToken)
	appRoutes.Register("/admins", "GET", callGetAdminList)
	appRoutes.Register("/create-admin", "POST", callAdminCreate)
	appRoutes.Register("/update-admin", "POST", callUpdateAdmin)
	appRoutes.Register("/delete-admin", "POST", callDeleteAdmin)
	appRoutes.Register("/logout", "POST", callAdminLogout)
	appRoutes.Register("/change-password", "POST", callAdminChangePassword)

	// 站点配置相关
	appRoutes.Register("/basic-setting", "GET", callGetBasicSetting)
	appRoutes.Register("/update-basic-setting", "POST", callUpdateBasicSetting)

	// 角色权限相关
	appRoutes.Register("/roles", "GET", callGetRoleList)
	appRoutes.Register("/create-role", "POST", callCreateRole)
	appRoutes.Register("/update-role", "POST", callUpdateRole)
	appRoutes.Register("/delete-role", "POST", callDeleteRole)
	appRoutes.Register("/permissions", "GET", callGetPermissions)
	appRoutes.Register("/save-permission", "POST", callSavePermission)

	// 日志相关
	appRoutes.Register("/admin-logs", "GET", callGetAdminLogs)

	// 用户相关
	appRoutes.Register("/users", "GET", callGetUserList)
	appRoutes.Register("/update-user", "POST", callUpdateUser)
	appRoutes.Register("/user-basic-info", "GET", callGetUserBasicInfo)
	appRoutes.Register("/user-grades", "GET", callGetUserGrades)
	appRoutes.Register("/save-user-grades", "POST", callSaveUserGrades)
	appRoutes.Register("/delete-user-grades", "POST", callDeleteUserGrades)
	appRoutes.Register("/user-login-logs", "GET", callGetUserLoginLogs)

	// 文件上传
	appRoutes.Register("/upload-image", "POST", callUploadImage)

	// 消息相关
	appRoutes.Register("/messages", "GET", callGetMessageList)
	appRoutes.Register("/create-message", "POST", callCreateMessage)
	appRoutes.Register("/user-messages", "GET", callGetUserMessages)
	appRoutes.Register("/unread-count", "GET", callGetUnreadCount)
	appRoutes.Register("/read-message", "POST", callReadMessage)

	// 广告相关
	appRoutes.Register("/ads", "GET", callGetAdList)
	appRoutes.Register("/create-ad", "POST", callCreateAd)
	appRoutes.Register("/update-ad", "POST", callUpdateAd)
	appRoutes.Register("/delete-ad", "POST", callDeleteAd)

	// 公告相关
	appRoutes.Register("/notices", "GET", callGetNoticeList)
	appRoutes.Register("/create-notice", "POST", callCreateNotice)
	appRoutes.Register("/update-notice", "POST", callUpdateNotice)
	appRoutes.Register("/delete-notice", "POST", callDeleteNotice)
}

// initBalanceRoutes 初始化余额路由
func initBalanceRoutes() {
	balanceRoutes = NewRouteRegistry()

	// 账变记录
	balanceRoutes.Register("/balance-changes", "GET", callGetBalanceChanges)

	// 充值相关
	balanceRoutes.Register("/recharge-payment", "GET", callGetRechargePayments)
	balanceRoutes.Register("/recharge-manual", "GET", callGetRechargeManuals)
	balanceRoutes.Register("/confirm-payment-order", "POST", callConfirmPaymentOrder)

	// 提现相关
	balanceRoutes.Register("/withdraws", "GET", callGetWithdraws)
	balanceRoutes.Register("/withdraw-manual", "GET", callGetWithdrawManuals)
	balanceRoutes.Register("/withdraw-review", "GET", callGetWithdrawReview)
	balanceRoutes.Register("/deal-with-withdraw", "POST", callDealWithWithdraw)

	// 余额查询和操作
	balanceRoutes.Register("/query-user-balance", "POST", callQueryUserBalance)
	balanceRoutes.Register("/query-game-balance", "POST", callQueryGameBalance)
	balanceRoutes.Register("/manual-user-balance", "POST", callManualUserBalance)

	// 支付接口管理
	balanceRoutes.Register("/payment-accounts", "GET", callGetPaymentAccounts)
	balanceRoutes.Register("/create-payment-account", "POST", callCreatePaymentAccount)
	balanceRoutes.Register("/update-payment-account", "GET", callGetPaymentAccountUpdate)
	balanceRoutes.Register("/update-payment-account", "POST", callUpdatePaymentAccount)
	balanceRoutes.Register("/delete-payment-account", "POST", callDeletePaymentAccount)
}

// initGameRoutes 初始化游戏路由
func initGameRoutes() {
	gameRoutes = NewRouteRegistry()

	// 游戏管理
	gameRoutes.Register("/games", "GET", callGetGames)
	gameRoutes.Register("/update-games", "POST", callUpdateGames)
	gameRoutes.Register("/game-record", "POST", callGetGameRecords)
}
