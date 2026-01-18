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
	appRoutes      *RouteRegistry
	balanceRoutes  *RouteRegistry
	gameRoutes     *RouteRegistry
	manageRoutes   *RouteRegistry
	frontendRoutes *RouteRegistry
)

// 初始化路由注册表
func init() {
	initAppRoutes()
	initBalanceRoutes()
	initGameRoutes()
	initManageRoutes()
	initFrontendRoutes()
}

// initAppRoutes 初始化应用路由
func initAppRoutes() {
	appRoutes = NewRouteRegistry()

	// 管理员登录
	appRoutes.Register("/login", "POST", callAdminLogin)
	// 刷新token
	appRoutes.Register("/refresh-token", "GET", callAdminRefreshToken)
	// 获取管理员信息
	appRoutes.Register("/get-info", "GET", callGetInfo)
	// 获取菜单列表
	appRoutes.Register("/menus", "GET", callMenus)
	// 员工列表
	appRoutes.Register("/admins", "GET", callGetAdminList)
	// 添加员工
	appRoutes.Register("/create-admin", "POST", callAdminCreate)
	// 更新员工
	appRoutes.Register("/update-admin", "POST", callUpdateAdmin)
	// 删除员工
	appRoutes.Register("/delete-admin", "POST", callDeleteAdmin)
	// 退出
	appRoutes.Register("/logout", "POST", callAdminLogout)
	// 修改密码
	appRoutes.Register("/change-password", "POST", callAdminChangePassword)

	// 站点配置信息
	appRoutes.Register("/basic-setting", "GET", callGetBasicSetting)
	// 设置站点配置信息
	appRoutes.Register("/update-basic-setting", "POST", callUpdateBasicSetting)

	// 角色列表
	appRoutes.Register("/roles", "GET", callGetRoleList)
	// 创建角色
	appRoutes.Register("/create-role", "POST", callCreateRole)
	// 更新角色
	appRoutes.Register("/update-role", "POST", callUpdateRole)
	// 删除角色
	appRoutes.Register("/delete-role", "POST", callDeleteRole)
	// 角色权限列表
	appRoutes.Register("/permissions", "GET", callGetPermissions)
	// 保存角色权限
	appRoutes.Register("/save-permission", "POST", callSavePermission)

	// 后台操作日志
	appRoutes.Register("/admin-logs", "GET", callGetAdminLogs)

	// 会员列表
	appRoutes.Register("/users", "GET", callGetUserList)
	// 更新会员
	appRoutes.Register("/update-user", "POST", callUpdateUser)
	// 会员基本信息
	appRoutes.Register("/user-basic-info", "GET", callGetUserBasicInfo)
	// 会员等级列表
	appRoutes.Register("/user-grades", "GET", callGetUserGrades)
	// 保存会员等级
	appRoutes.Register("/save-user-grades", "POST", callSaveUserGrades)
	// 删除会员等级
	appRoutes.Register("/delete-user-grades", "POST", callDeleteUserGrades)
	//会员登录日志
	appRoutes.Register("/user-login-logs", "GET", callGetUserLoginLogs)

	// 图片上传
	appRoutes.Register("/upload-image", "POST", callUploadImage)

	// 消息列表
	appRoutes.Register("/messages", "GET", callGetMessageList)
	// 新增消息
	appRoutes.Register("/create-message", "POST", callCreateMessage)
	// 用户-消息列表(预留前台)
	appRoutes.Register("/user-messages", "GET", callGetUserMessages)
	// 用户-已读消息(预留前台)
	appRoutes.Register("/unread-count", "GET", callGetUnreadCount)
	// 用户-未读消息(预留前台)
	appRoutes.Register("/read-message", "POST", callReadMessage)

	// 广告列表
	appRoutes.Register("/ads", "GET", callGetAdList)
	// 新增广告
	appRoutes.Register("/create-ad", "POST", callCreateAd)
	// 更新广告
	appRoutes.Register("/update-ad", "POST", callUpdateAd)
	// 删除广告
	appRoutes.Register("/delete-ad", "POST", callDeleteAd)

	// 公告列表
	appRoutes.Register("/notices", "GET", callGetNoticeList)
	// 新增公告
	appRoutes.Register("/create-notice", "POST", callCreateNotice)
	// 更新公告
	appRoutes.Register("/update-notice", "POST", callUpdateNotice)
	// 删除公告
	appRoutes.Register("/delete-notice", "POST", callDeleteNotice)

	// 选项接口
	// 会员等级列表
	appRoutes.Register("/options-user-grade", "GET", callGetUserGradeOptions)
	// 后台角色列表
	appRoutes.Register("/options-admin-role", "GET", callGetAdminRoleOptions)
}

// initBalanceRoutes 初始化余额路由
func initBalanceRoutes() {
	balanceRoutes = NewRouteRegistry()

	// 账变记录
	balanceRoutes.Register("/balance-changes", "GET", callGetBalanceChanges)
	// 交易类型列表
	balanceRoutes.Register("/options-change", "GET", callGetChangeOptions)

	// 充值记录
	balanceRoutes.Register("/recharge-payment", "GET", callGetRechargePayments)
	// 手动充值记录
	balanceRoutes.Register("/recharge-manual", "GET", callGetRechargeManuals)
	// 确认充值
	balanceRoutes.Register("/confirm-payment-order", "POST", callConfirmPaymentOrder)

	// 提现记录
	balanceRoutes.Register("/withdraws", "GET", callGetWithdraws)
	// 手动提现记录
	balanceRoutes.Register("/withdraw-manual", "GET", callGetWithdrawManuals)
	// 提现查看审核
	balanceRoutes.Register("/withdraw-review", "GET", callGetWithdrawReview)
	// 确认提现
	balanceRoutes.Register("/deal-with-withdraw", "POST", callDealWithWithdraw)

	// 用户余额查询
	balanceRoutes.Register("/query-user-balance", "POST", callQueryUserBalance)
	// 用户三方余额查询
	balanceRoutes.Register("/query-game-balance", "POST", callQueryGameBalance)
	// 手动操作用户账户
	balanceRoutes.Register("/manual-user-balance", "POST", callManualUserBalance)

	// 支付接口列表
	balanceRoutes.Register("/payment-accounts", "GET", callGetPaymentAccounts)
	// 添加支付接口
	balanceRoutes.Register("/create-payment-account", "POST", callCreatePaymentAccount)
	// 更新获取支付接口信息
	balanceRoutes.Register("/update-payment-account", "GET", callGetPaymentAccountUpdate)
	// 更新支付接口
	balanceRoutes.Register("/update-payment-account", "POST", callUpdatePaymentAccount)
	// 删除支付接口
	balanceRoutes.Register("/delete-payment-account", "POST", callDeletePaymentAccount)

	// 操作类型列表
	balanceRoutes.Register("/options-manual", "GET", callGetManualList)
}

// initGameRoutes 初始化游戏路由
func initGameRoutes() {
	gameRoutes = NewRouteRegistry()

	// 获取游戏列表
	gameRoutes.Register("/games", "GET", callGetGames)
	// 修改站点游戏开关
	gameRoutes.Register("/update-games", "POST", callUpdateGames)
	// 获取游戏记录
	gameRoutes.Register("/game-record", "POST", callGetGameRecords)
}

// initManageRoutes 初始化总后台管理路由
func initManageRoutes() {
	manageRoutes = NewRouteRegistry()

	// 管理员认证接口
	manageRoutes.Register("/admin/login", "POST", callManageAdminLogin)
	manageRoutes.Register("/admin/refresh-token", "GET", callManageAdminRefreshToken)
	manageRoutes.Register("/admin/get-info", "GET", callManageAdminGetInfo)
	manageRoutes.Register("/admin/logout", "POST", callManageAdminLogout)
	manageRoutes.Register("/admin/menus", "GET", callManageAdminMenus)

	// 商户管理接口
	manageRoutes.Register("/customer/list", "GET", callGetCustomerList)
	manageRoutes.Register("/customer/detail", "GET", callGetCustomerDetail)
	manageRoutes.Register("/customer/update", "POST", callUpdateCustomer)
	manageRoutes.Register("/customer/delete", "POST", callDeleteCustomer)
	manageRoutes.Register("/customer/sync", "POST", callSyncCustomer)
	manageRoutes.Register("/customer/publish", "POST", callPublishCustomer)
	manageRoutes.Register("/customer/rollback", "POST", callRollbackCustomer)
	manageRoutes.Register("/customer/version-history", "GET", callGetCustomerVersionHistory)

	// 选项接口
	manageRoutes.Register("/options/domain-type", "GET", callGetDomainTypeOptions)
	manageRoutes.Register("/options/site", "GET", callGetSiteOptions)
	manageRoutes.Register("/site-domain/list", "GET", callGetSiteDomainList)
	manageRoutes.Register("/site-domain/create", "POST", callCreateSiteDomain)
	manageRoutes.Register("/site-domain/update", "POST", callUpdateSiteDomain)
	manageRoutes.Register("/site-domain/delete", "POST", callDeleteSiteDomain)
}

// initFrontendRoutes 初始化前台路由
func initFrontendRoutes() {
	frontendRoutes = NewRouteRegistry()

	// 快速注册
	frontendRoutes.Register("/quickRegister", "POST", callQuickRegister)
	// 用户注册
	frontendRoutes.Register("/register", "POST", callUserRegister)
	// 用户登录
	frontendRoutes.Register("/login", "POST", callUserLogin)
	// 获取验证码
	frontendRoutes.Register("/captcha", "GET", callGenerateCaptcha)
	// 刷新token
	frontendRoutes.Register("/refresh-token", "POST", callRefreshToken)
	// 获取用户信息
	frontendRoutes.Register("/user-info", "GET", callGetUserInfo)
	// 用户退出
	frontendRoutes.Register("/logout", "POST", callUserLogout)
	// 绑定邮箱
	frontendRoutes.Register("/bind-email", "POST", callBindEmail)
	// 绑定手机
	frontendRoutes.Register("/bind-mobile", "POST", callBindMobile)
	// 修改登录密码
	frontendRoutes.Register("/change-login-password", "POST", callChangeLoginPassword)
	// 绑定登录密码（针对快速注册用户）
	frontendRoutes.Register("/bind-login-password", "POST", callBindLoginPassword)
}
