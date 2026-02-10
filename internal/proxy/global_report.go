package proxy

import (
	"context"
	"time"

	"jh_gateway/internal/middleware"
	"jh_gateway/internal/registry"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
)

const reportTimeLayout = "2006-01-02 15:04:05"

// HandleGlobalReport 处理后台全局报表
func HandleGlobalReport(ctx context.Context, r *ghttp.Request) {
	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求参数解析失败")
		return
	}

	startStr := gconv.String(reqData["start_time"])
	endStr := gconv.String(reqData["end_time"])
	if startStr == "" {
		startStr = gconv.String(reqData["startTime"])
	}
	if endStr == "" {
		endStr = gconv.String(reqData["endTime"])
	}
	if startStr == "" || endStr == "" {
		util.WriteBadRequest(r, "开始时间和结束时间不能为空")
		return
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	startTime, err := time.ParseInLocation(reportTimeLayout, startStr, loc)
	if err != nil {
		util.WriteBadRequest(r, "开始时间格式错误")
		return
	}
	endTime, err := time.ParseInLocation(reportTimeLayout, endStr, loc)
	if err != nil {
		util.WriteBadRequest(r, "结束时间格式错误")
		return
	}
	if endTime.Before(startTime) {
		util.WriteBadRequest(r, "结束时间不能早于开始时间")
		return
	}

	siteID := middleware.GetSiteIDFromRequest(r)
	if siteID == 0 {
		siteID = gconv.Int64(reqData["site_id"])
	}
	if siteID == 0 {
		util.WriteBadRequest(r, "站点ID不能为空")
		return
	}

	if err := registry.InitClickHouse(ctx); err != nil {
		util.WriteInternalError(r, "ClickHouse连接失败")
		return
	}

	conn := registry.GetClickHouseManager().GetConn()
	if conn == nil {
		util.WriteInternalError(r, "ClickHouse连接未初始化")
		return
	}

	var (
		rechargeUsers        uint64
		depositOrders        uint64
		depositSuccessOrders uint64
		depositTotal         float64
		withdrawTotal        float64
		bonusTotal           float64
		rebateTotal          float64
	)

	balanceLogQuery := `
WITH bl AS (
	SELECT
		id,
		argMax(user_id, event_id) AS user_id,
		argMax(trade_type, event_id) AS trade_type,
		argMax(status, event_id) AS status,
		argMax(money, event_id) AS money,
		argMax(actual_money, event_id) AS actual_money
	FROM game_analytics.balance_log
	WHERE site_id = ? AND created_at >= ? AND created_at <= ?
	GROUP BY id
)
SELECT
	countDistinctIf(user_id, status = 3 AND trade_type IN (1,2,3,6)) AS recharge_users,
	countIf(trade_type IN (1,2,3)) AS deposit_orders,
	countIf(status = 3 AND trade_type IN (1,2,3)) AS deposit_success_orders,
	ifNull(toFloat64(sumIf(if(actual_money > 0, actual_money, money), status = 3 AND trade_type IN (1,2,3))), 0) AS deposit_total,
	ifNull(toFloat64(sumIf(if(actual_money > 0, actual_money, money), status = 3 AND trade_type IN (4,5))), 0) AS withdraw_total,
	ifNull(toFloat64(sumIf(if(actual_money > 0, actual_money, money), status = 3 AND trade_type = 8)), 0) AS bonus_total,
	ifNull(toFloat64(sumIf(if(actual_money > 0, actual_money, money), status = 3 AND trade_type = 9)), 0) AS rebate_total
FROM bl
`

	if err := conn.QueryRow(ctx, balanceLogQuery, siteID, startTime, endTime).
		Scan(&rechargeUsers, &depositOrders, &depositSuccessOrders, &depositTotal, &withdrawTotal, &bonusTotal, &rebateTotal); err != nil {
		g.Log().Errorf(ctx, "查询balance_log统计失败: %v", err)
		util.WriteInternalError(r, "统计查询失败")
		return
	}

	var (
		firstDepositUsers  uint64
		firstDepositAmount float64
	)
	firstDepositQuery := `
WITH bl_latest AS (
	SELECT
		id,
		argMax(user_id, event_id) AS user_id,
		argMax(trade_type, event_id) AS trade_type,
		argMax(status, event_id) AS status,
		argMax(money, event_id) AS money,
		argMax(actual_money, event_id) AS actual_money,
		argMax(created_at, event_id) AS created_at
	FROM game_analytics.balance_log
	WHERE site_id = ?
	GROUP BY id
),
firsts AS (
	SELECT
		user_id,
		min(created_at) AS first_time,
		argMin(if(actual_money > 0, actual_money, money), created_at) AS first_amount
	FROM bl_latest
	WHERE status = 3 AND trade_type IN (1,2,3,6)
	GROUP BY user_id
)
SELECT
	countIf(first_time >= ? AND first_time <= ?) AS first_deposit_users,
	ifNull(toFloat64(sumIf(first_amount, first_time >= ? AND first_time <= ?)), 0) AS first_deposit_amount
FROM firsts
`

	if err := conn.QueryRow(ctx, firstDepositQuery, siteID, startTime, endTime, startTime, endTime).
		Scan(&firstDepositUsers, &firstDepositAmount); err != nil {
		g.Log().Errorf(ctx, "查询首存统计失败: %v", err)
		util.WriteInternalError(r, "统计查询失败")
		return
	}

	var (
		validBetAmount float64
		netAmount      float64
	)
	betQuery := `
SELECT
	ifNull(toFloat64(sum(valid_amount)), 0) AS valid_amount,
	ifNull(toFloat64(sum(net_amount)), 0) AS net_amount
FROM game_analytics.bet_records
WHERE site_id = ? AND bet_time >= ? AND bet_time <= ?
`

	if err := conn.QueryRow(ctx, betQuery, siteID, startTime, endTime).
		Scan(&validBetAmount, &netAmount); err != nil {
		g.Log().Errorf(ctx, "查询投注统计失败: %v", err)
		util.WriteInternalError(r, "统计查询失败")
		return
	}

	depositSuccessRate := 0.0
	if depositOrders > 0 {
		depositSuccessRate = (float64(depositSuccessOrders) / float64(depositOrders)) * 100
	}

	util.WriteSuccess(r, map[string]interface{}{
		"recharge_users":         rechargeUsers,
		"first_deposit_users":    firstDepositUsers,
		"first_deposit_amount":   firstDepositAmount,
		"valid_bet_amount":       validBetAmount,
		"net_amount":             netAmount,
		"deposit_total":          depositTotal,
		"withdraw_total":         withdrawTotal,
		"bonus_total":            bonusTotal,
		"rebate_total":           rebateTotal,
		"deposit_success_orders": depositSuccessOrders,
		"deposit_orders":         depositOrders,
		"deposit_success_rate":   depositSuccessRate,
	})
}
