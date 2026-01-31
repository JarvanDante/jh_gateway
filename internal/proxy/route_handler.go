package proxy

import (
	"context"
	"fmt"
	"strings"

	"jh_gateway/internal/proxy/grpc_client/backend"
	"jh_gateway/internal/proxy/grpc_client/balance"
	"jh_gateway/internal/proxy/grpc_client/frontend"
	"jh_gateway/internal/proxy/grpc_client/game"
	"jh_gateway/internal/proxy/grpc_client/manage"

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
	backendRoutes  *RouteRegistry
	balanceRoutes  *RouteRegistry
	gameRoutes     *RouteRegistry
	manageRoutes   *RouteRegistry
	frontendRoutes *RouteRegistry
)

// 初始化路由注册表
func init() {
	initBackendRoutes()
	initBalanceRoutes()
	initGameRoutes()
	initManageRoutes()
	initFrontendRoutes()
}

// initBackendRoutes 初始化后台路由（后台）
func initBackendRoutes() {
	backendRoutes = NewRouteRegistry()

	// 管理员登录
	backendRoutes.Register("/login", "POST", backend.CallAdminLogin)
	// 刷新token
	backendRoutes.Register("/refresh-token", "GET", backend.CallAdminRefreshToken)
	// 获取管理员信息
	backendRoutes.Register("/get-info", "GET", backend.CallGetInfo)
	// 获取菜单列表
	backendRoutes.Register("/menus", "GET", backend.CallMenus)
	// 员工列表
	backendRoutes.Register("/admins", "GET", backend.CallGetAdminList)
	// 添加员工
	backendRoutes.Register("/create-admin", "POST", backend.CallAdminCreate)
	// 更新员工
	backendRoutes.Register("/update-admin", "POST", backend.CallUpdateAdmin)
	// 删除员工
	backendRoutes.Register("/delete-admin", "POST", backend.CallDeleteAdmin)
	// 退出
	backendRoutes.Register("/logout", "POST", backend.CallAdminLogout)
	// 修改密码
	backendRoutes.Register("/change-password", "POST", backend.CallAdminChangePassword)

	// 站点配置信息
	backendRoutes.Register("/basic-setting", "GET", backend.CallGetBasicSetting)
	// 设置站点配置信息
	backendRoutes.Register("/update-basic-setting", "POST", backend.CallUpdateBasicSetting)

	// 角色列表
	backendRoutes.Register("/roles", "GET", backend.CallGetRoleList)
	// 创建角色
	backendRoutes.Register("/create-role", "POST", backend.CallCreateRole)
	// 更新角色
	backendRoutes.Register("/update-role", "POST", backend.CallUpdateRole)
	// 删除角色
	backendRoutes.Register("/delete-role", "POST", backend.CallDeleteRole)
	// 角色权限列表
	backendRoutes.Register("/permissions", "GET", backend.CallGetPermissions)
	// 保存角色权限
	backendRoutes.Register("/save-permission", "POST", backend.CallSavePermission)

	// 后台操作日志
	backendRoutes.Register("/admin-logs", "GET", backend.CallGetAdminLogs)

	// 会员列表
	backendRoutes.Register("/users", "GET", backend.CallGetUserList)
	// 更新会员
	backendRoutes.Register("/update-user", "POST", backend.CallUpdateUser)
	// 会员基本信息
	backendRoutes.Register("/user-basic-info", "GET", backend.CallGetUserBasicInfo)
	// 会员等级列表
	backendRoutes.Register("/user-grades", "GET", backend.CallGetUserGrades)
	// 保存会员等级
	backendRoutes.Register("/save-user-grades", "POST", backend.CallSaveUserGrades)
	// 删除会员等级
	backendRoutes.Register("/delete-user-grades", "POST", backend.CallDeleteUserGrades)
	//会员登录日志
	backendRoutes.Register("/user-login-logs", "GET", backend.CallGetUserLoginLogs)

	// 图片上传
	backendRoutes.Register("/upload-image", "POST", backend.CallUploadImage)

	// 消息列表
	backendRoutes.Register("/messages", "GET", backend.CallGetMessageList)
	// 新增消息
	backendRoutes.Register("/create-message", "POST", backend.CallCreateMessage)
	// 用户-消息列表(预留前台)
	backendRoutes.Register("/user-messages", "GET", backend.CallGetUserMessages)
	// 用户-已读消息(预留前台)
	backendRoutes.Register("/unread-count", "GET", backend.CallGetUnreadCount)
	// 用户-未读消息(预留前台)
	backendRoutes.Register("/read-message", "POST", backend.CallReadMessage)

	// 广告列表
	backendRoutes.Register("/ads", "GET", backend.CallGetAdList)
	// 新增广告
	backendRoutes.Register("/create-ad", "POST", backend.CallCreateAd)
	// 更新广告
	backendRoutes.Register("/update-ad", "POST", backend.CallUpdateAd)
	// 删除广告
	backendRoutes.Register("/delete-ad", "POST", backend.CallDeleteAd)

	// 公告列表
	backendRoutes.Register("/notices", "GET", backend.CallGetNoticeList)
	// 新增公告
	backendRoutes.Register("/create-notice", "POST", backend.CallCreateNotice)
	// 更新公告
	backendRoutes.Register("/update-notice", "POST", backend.CallUpdateNotice)
	// 删除公告
	backendRoutes.Register("/delete-notice", "POST", backend.CallDeleteNotice)

	// 选项接口
	// 会员等级列表
	backendRoutes.Register("/options-user-grade", "GET", backend.CallGetUserGradeOptions)
	// 后台角色列表
	backendRoutes.Register("/options-admin-role", "GET", backend.CallGetAdminRoleOptions)
}

// initBalanceRoutes 初始化余额路由
func initBalanceRoutes() {
	balanceRoutes = NewRouteRegistry()

	// 前台接口
	// 刷新用户余额
	balanceRoutes.Register("/refresh-balance", "GET", balance.CallRefreshBalance)
	// 获取支付渠道列表
	balanceRoutes.Register("/payment-list", "GET", balance.CallGetPaymentList)
	// 获取提现渠道列表
	balanceRoutes.Register("/withdraw-list", "GET", balance.CallGetWithdrawList)
	// 银行卡管理
	balanceRoutes.Register("/add-bank-card", "POST", balance.CallAddBankCard)
	balanceRoutes.Register("/bank-card-list", "GET", balance.CallGetBankCardList)
	balanceRoutes.Register("/delete-bank-card", "POST", balance.CallDeleteBankCard)
	balanceRoutes.Register("/set-default-bank-card", "POST", balance.CallSetDefaultBankCard)
	balanceRoutes.Register("/deposit-nonce", "GET", balance.CallGetDepositNonce)
	balanceRoutes.Register("/withdraw-nonce", "GET", balance.CallGetWithdrawNonce)
	balanceRoutes.Register("/deposit-order", "POST", balance.CallCreateDepositOrder)
	// 账户明细
	balanceRoutes.Register("/balance-log", "GET", balance.CallGetBalanceLog)

	// 后台接口
	// 账变记录
	balanceRoutes.Register("/balance-changes", "GET", balance.CallGetBalanceChanges)
	// 交易类型列表
	balanceRoutes.Register("/options-change", "GET", balance.CallGetChangeOptions)

	// 充值记录
	balanceRoutes.Register("/recharge-payment", "GET", balance.CallGetRechargePayments)
	// 手动充值记录
	balanceRoutes.Register("/recharge-manual", "GET", balance.CallGetRechargeManuals)
	// 确认充值
	balanceRoutes.Register("/confirm-payment-order", "POST", balance.CallConfirmPaymentOrder)

	// 提现记录
	balanceRoutes.Register("/withdraws", "GET", balance.CallGetWithdraws)
	// 手动提现记录
	balanceRoutes.Register("/withdraw-manual", "GET", balance.CallGetWithdrawManuals)
	// 提现查看审核
	balanceRoutes.Register("/withdraw-review", "GET", balance.CallGetWithdrawReview)
	// 确认提现
	balanceRoutes.Register("/deal-with-withdraw", "POST", balance.CallDealWithWithdraw)

	// 用户余额查询
	balanceRoutes.Register("/query-user-balance", "POST", balance.CallQueryUserBalance)
	// 用户三方余额查询
	balanceRoutes.Register("/query-game-balance", "POST", balance.CallQueryGameBalance)
	// 手动操作用户账户
	balanceRoutes.Register("/manual-user-balance", "POST", balance.CallManualUserBalance)

	// 操作类型列表
	balanceRoutes.Register("/options-manual", "GET", balance.CallGetManualList)

	// 站点支付管理
	balanceRoutes.Register("/site-payments", "GET", balance.CallGetSitePayments)
	balanceRoutes.Register("/update-site-payment", "POST", balance.CallUpdateSitePayment)

	// 站点提现管理
	balanceRoutes.Register("/site-withdraws", "GET", balance.CallGetSiteWithdraws)
	balanceRoutes.Register("/update-site-withdraw", "POST", balance.CallUpdateSiteWithdraw)
}

// initGameRoutes 初始化游戏路由
func initGameRoutes() {
	gameRoutes = NewRouteRegistry()

	// 获取游戏种类列表
	gameRoutes.Register("/games", "GET", game.CallGetGames)
	// 修改站点游戏种类开关
	gameRoutes.Register("/update-games", "POST", game.CallUpdateGames)
	// 获取游戏记录
	gameRoutes.Register("/game-record", "POST", game.CallGetGameRecords)
	// 进入游戏
	gameRoutes.Register("/redirect-to-game", "POST", game.CallRedirectToGame)
	// 获取游戏种类（site_game表）
	gameRoutes.Register("/game-categories", "GET", game.CallGetGameCategories)
	// 获取游戏列表（game_list表）- 前台
	gameRoutes.Register("/frontend-game-list", "GET", game.CallGetFrontendGameList)
	// 获取游戏列表（game_list表）- 后台
	gameRoutes.Register("/backend-game-list", "GET", game.CallGetBackendGameList)
	// 更新游戏列表项
	gameRoutes.Register("/update-game-list-item", "POST", game.CallUpdateBackendGameListItem)
	// 获取游戏平台列表
	gameRoutes.Register("/game-platforms", "GET", game.CallGetGamePlatforms)
}

// initManageRoutes 初始化总后台管理路由
func initManageRoutes() {
	manageRoutes = NewRouteRegistry()

	// 管理员认证接口
	manageRoutes.Register("/admin/login", "POST", manage.CallManageAdminLogin)
	manageRoutes.Register("/admin/refresh-token", "GET", manage.CallManageAdminRefreshToken)
	manageRoutes.Register("/admin/get-info", "GET", manage.CallManageAdminGetInfo)
	manageRoutes.Register("/admin/logout", "POST", manage.CallManageAdminLogout)
	manageRoutes.Register("/admin/menus", "GET", manage.CallManageAdminMenus)

	// 商户管理接口
	manageRoutes.Register("/customer/list", "GET", manage.CallGetCustomerList)
	manageRoutes.Register("/customer/detail", "GET", manage.CallGetCustomerDetail)
	manageRoutes.Register("/customer/update", "POST", manage.CallUpdateCustomer)
	manageRoutes.Register("/customer/delete", "POST", manage.CallDeleteCustomer)
	manageRoutes.Register("/customer/sync", "POST", manage.CallSyncCustomer)
	manageRoutes.Register("/customer/publish", "POST", manage.CallPublishCustomer)
	manageRoutes.Register("/customer/rollback", "POST", manage.CallRollbackCustomer)
	manageRoutes.Register("/customer/version-history", "GET", manage.CallGetCustomerVersionHistory)

	// 游戏管理接口
	manageRoutes.Register("/game/list", "GET", manage.CallGetGameList)
	manageRoutes.Register("/game/detail", "GET", manage.CallGetGameDetail)
	manageRoutes.Register("/game/create", "POST", manage.CallCreateGame)
	manageRoutes.Register("/game/update", "POST", manage.CallUpdateGame)
	manageRoutes.Register("/game/delete", "POST", manage.CallDeleteGame)
	manageRoutes.Register("/game/supplement-bet-records", "POST", manage.CallSupplementBetRecords)
	manageRoutes.Register("/game/sync-tasks", "GET", manage.CallManageGetSyncTasks)

	// 支付管理接口
	manageRoutes.Register("/payment/list", "GET", manage.CallGetPaymentList)
	manageRoutes.Register("/payment/detail", "GET", manage.CallGetPaymentDetail)
	manageRoutes.Register("/payment/create", "POST", manage.CallCreatePayment)
	manageRoutes.Register("/payment/update", "POST", manage.CallUpdatePayment)
	manageRoutes.Register("/payment/delete", "POST", manage.CallDeletePayment)

	// 提现管理接口
	manageRoutes.Register("/withdraw/list", "GET", manage.CallGetWithdrawList)
	manageRoutes.Register("/withdraw/detail", "GET", manage.CallGetWithdrawDetail)
	manageRoutes.Register("/withdraw/create", "POST", manage.CallCreateWithdraw)
	manageRoutes.Register("/withdraw/update", "POST", manage.CallUpdateWithdraw)
	manageRoutes.Register("/withdraw/delete", "POST", manage.CallDeleteWithdraw)

	// 站点游戏管理接口
	manageRoutes.Register("/site_game/list", "GET", manage.CallGetSiteGameList)
	manageRoutes.Register("/site_game/update_status", "POST", manage.CallUpdateSiteGameStatus)
	manageRoutes.Register("/site_game/bind", "POST", manage.CallBindSiteGame)
	manageRoutes.Register("/site_game/unbind", "POST", manage.CallUnbindSiteGame)

	// 站点支付管理接口
	manageRoutes.Register("/site_payment/list", "GET", manage.CallGetSitePaymentList)
	manageRoutes.Register("/site_payment/update_status", "POST", manage.CallUpdateSitePaymentStatus)
	manageRoutes.Register("/site_payment/bind", "POST", manage.CallBindSitePayment)
	manageRoutes.Register("/site_payment/unbind", "POST", manage.CallUnbindSitePayment)

	// 站点提现管理接口
	manageRoutes.Register("/site_withdraw/list", "GET", manage.CallGetSiteWithdrawList)
	manageRoutes.Register("/site_withdraw/update_status", "POST", manage.CallUpdateSiteWithdrawStatus)
	manageRoutes.Register("/site_withdraw/bind", "POST", manage.CallBindSiteWithdraw)
	manageRoutes.Register("/site_withdraw/unbind", "POST", manage.CallUnbindSiteWithdraw)

	// 选项接口
	manageRoutes.Register("/options/domain-type", "GET", manage.CallGetDomainTypeOptions)
	manageRoutes.Register("/options/site", "GET", manage.CallGetSiteOptions)
	manageRoutes.Register("/site-domain/list", "GET", manage.CallGetSiteDomainList)
	manageRoutes.Register("/site-domain/create", "POST", manage.CallCreateSiteDomain)
	manageRoutes.Register("/site-domain/update", "POST", manage.CallUpdateSiteDomain)
	manageRoutes.Register("/site-domain/delete", "POST", manage.CallDeleteSiteDomain)
}

// initFrontendRoutes 初始化前台路由
func initFrontendRoutes() {
	frontendRoutes = NewRouteRegistry()

	// 快速注册
	frontendRoutes.Register("/quickRegister", "POST", frontend.CallQuickRegister)
	// 用户注册
	frontendRoutes.Register("/register", "POST", frontend.CallUserRegister)
	// 用户登录
	frontendRoutes.Register("/login", "POST", frontend.CallUserLogin)
	// 获取验证码
	frontendRoutes.Register("/captcha", "GET", frontend.CallGenerateCaptcha)
	// 刷新token
	frontendRoutes.Register("/refresh-token", "POST", frontend.CallRefreshToken)
	// 获取用户信息
	frontendRoutes.Register("/user-info", "GET", frontend.CallGetUserInfo)
	// 用户退出
	frontendRoutes.Register("/logout", "POST", frontend.CallUserLogout)
	// 绑定邮箱
	frontendRoutes.Register("/bind-email", "POST", frontend.CallBindEmail)
	// 绑定手机
	frontendRoutes.Register("/bind-mobile", "POST", frontend.CallBindMobile)
	// 修改登录密码
	frontendRoutes.Register("/change-login-password", "POST", frontend.CallChangeLoginPassword)
	// 绑定登录密码（针对快速注册用户）
	frontendRoutes.Register("/bind-login-password", "POST", frontend.CallBindLoginPassword)
	// 获取会员等级列表
	frontendRoutes.Register("/grade-list", "GET", frontend.CallGetAppGradeList)
	// 设置支付密码
	frontendRoutes.Register("/set-pay-password", "POST", frontend.CallSetPayPassword)
	// 验证支付密码
	frontendRoutes.Register("/verify-pay-password", "POST", frontend.CallVerifyPayPassword)
	// 获取公告列表
	frontendRoutes.Register("/notice-list", "GET", frontend.CallGetAppNoticeList)
	// 获取广告列表
	frontendRoutes.Register("/ad-list", "GET", frontend.CallGetAppAdList)
	// 获取站点设置
	frontendRoutes.Register("/setting-site", "GET", frontend.CallGetSiteSetting)
}
