package game

import (
	"context"
	"fmt"
	"strconv"

	v7 "jh_gateway/api/backend/game/v1"
	v20 "jh_gateway/api/frontend/game/v1"
	"jh_gateway/internal/middleware"
	"jh_gateway/internal/util"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
	"google.golang.org/grpc"
)

// writeProtobufResponse 统一处理protobuf响应的JSON序列化
func writeProtobufResponse(ctx context.Context, r *ghttp.Request, pbMessage interface{}) error {
	util.WriteSuccess(r, pbMessage)
	return nil
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏种类
 * @title 获取游戏种类列表
 * @description 获取游戏种类列表的接口
 * @method get
 * @url /api/backend/game/games
 * @param seachBlance 可选 bool 是否查询余额
 * @return {"code":0,"msg":"success","data":{"count":4,"data":{"4":{"games":[{"code":"GFG_GAME","db_name":"gfg_game","id":15,"image_code":"nx46","name":"GFG电游","platform":"GFG","sort":1,"status":1,"type":4}]}}}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 1
 */
func CallGetGames(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGames")

	siteId := int32(1)
	searchBalance := r.Get("seachBlance").Bool()

	req := &v7.GetGamesReq{
		SiteId:        siteId,
		SearchBalance: searchBalance,
	}

	client := v7.NewGameClient(conn)
	res, err := client.GetGames(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGames failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏种类
 * @title 修改站点游戏种类开关
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
func CallUpdateGames(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game UpdateGames")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.LogWithTrace(ctx, "error", "failed to get request data: %v", err)
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	siteId := int32(1)
	dataInterface, exists := reqData["data"]
	if !exists {
		util.WriteBadRequest(r, "缺少data参数")
		return fmt.Errorf("missing data parameter")
	}

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

	req := &v7.UpdateGamesReq{
		SiteId: siteId,
		Data:   gameUpdates,
	}

	client := v7.NewGameClient(conn)
	res, err := client.UpdateGames(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game UpdateGames failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏记录
 * @title 获取游戏记录
 * @description 获取游戏记录的接口
 * @method post
 * @url /api/backend/game/game-record
 * @param platform 可选 string 游戏平台代码（不传则查询所有平台）
 * @param start_date 可选 string 开始时间
 * @param end_date 可选 string 结束时间
 * @param page 可选 int 页码
 * @param size 可选 int 每页数量
 * @param username 可选 string 用户名
 * @param type 可选 int 记录类型
 * @param keywords 可选 string 关键词搜索
 * @return {"code":0,"msg":"success","data":{}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 3
 */
func CallGetGameRecords(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGameRecords")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.LogWithTrace(ctx, "error", "failed to get request data: %v", err)
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	siteId := int32(1)
	platform := ""
	if p, exists := reqData["platform"]; exists {
		if pStr, ok := p.(string); ok {
			platform = pStr
		}
	}
	// platform 参数改为可选，不再强制校验

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

	client := v7.NewGameClient(conn)
	res, err := client.GetGameRecords(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGameRecords failed: %v", err)
		return err
	}

	// 按需返回字段，显式移除 total_amount/all_game_total_amount
	util.WriteSuccess(r, map[string]interface{}{
		"data":         res.GetData(),
		"total":        res.GetTotal(),
		"current_page": res.GetCurrentPage(),
		"per_page":     res.GetPerPage(),
		"last_page":    res.GetLastPage(),
	})
	return nil
}

/**
 * showdoc
 * @catalog 前台/游戏/游戏管理
 * @title 进入游戏
 * @description 获取游戏跳转链接的接口
 * @method post
 * @url /api/game/redirect-to-game
 * @param game_id 必选 int 游戏ID
 * @param key 必选 string 游戏代码
 * @param mobile 可选 int 是否移动端
 * @return {"code":0,"msg":"success","data":{"type":"url","url":"https://game.example.com/..."}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 4
 */
func CallRedirectToGame(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game RedirectToGame")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.LogWithTrace(ctx, "error", "failed to get request data: %v", err)
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	gameId := int32(0)
	if gid, exists := reqData["game_id"]; exists {
		switch v := gid.(type) {
		case float64:
			gameId = int32(v)
		case int:
			gameId = int32(v)
		case int32:
			gameId = v
		case int64:
			gameId = int32(v)
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 32); err == nil {
				gameId = int32(parsed)
			}
		}
	}
	if gameId <= 0 {
		util.WriteBadRequest(r, "缺少game_id参数或参数无效")
		return fmt.Errorf("missing or invalid game_id parameter")
	}

	key := ""
	if k, exists := reqData["key"]; exists {
		if kStr, ok := k.(string); ok {
			key = kStr
		}
	}

	mobile := int32(0)
	if m, exists := reqData["mobile"]; exists {
		switch v := m.(type) {
		case float64:
			mobile = int32(v)
		case int:
			mobile = int32(v)
		case int32:
			mobile = v
		case int64:
			mobile = int32(v)
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 32); err == nil {
				mobile = int32(parsed)
			}
		}
	}

	req := &v20.RedirectToGameReq{
		GameId: gameId,
		Key:    key,
		Mobile: mobile,
	}

	client := v20.NewGameClient(conn)
	res, err := client.RedirectToGame(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game RedirectToGame failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏资源管理
 * @title 获取游戏资源列表
 * @description 获取game_list表中的游戏资源列表，支持多条件筛选和分页
 * @method get
 * @url /api/backend/game/game-list
 * @param site_id 可选 int 站点ID
 * @param platform 可选 string 平台（ONE、IG_JILI等）
 * @param vendor 可选 string 运营商（PP、PGS、JL、JDB、CG等）
 * @param name 可选 string 游戏名称（模糊搜索）
 * @param code 可选 string 游戏代码
 * @param style 可选 string 游戏类型（SLOTS、LIVE等）
 * @param hot 可选 int 是否热门（-1=全部 0=否 1=是）
 * @param recommend 可选 int 是否推荐（-1=全部 0=否 1=是）
 * @param is_new 可选 int 是否新游戏（-1=全部 0=否 1=是）
 * @param status 可选 int 状态（-1=全部 0=禁用 1=启用）
 * @param page 可选 int 页码（默认1）
 * @param size 可选 int 每页数量（默认20，最大100）
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"site_id":1,"platform":"ONE","code":"vs20olympgate","name":"Gates of Olympus","alias_name":"奥林匹斯之门","style":"SLOTS","img":"https://example.com/game.png","img1":"","language":"en,zh","device":"MOBILE,DESKTOP","currency":"","vendor":"PP","hot":1,"recommend":1,"is_new":0,"status":1,"sort":100,"like_num":0,"created_at":"2026-01-25 17:57:33","updated_at":"2026-01-25 17:57:33"}],"total":1235,"page":1,"size":20}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 游戏列表
 * @return_param data.total int 总记录数
 * @return_param data.page int 当前页码
 * @return_param data.size int 每页数量
 * @number 7
 */
func CallGetBackendGameList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGameList")

	siteId := gconv.Int32(r.Get("site_id").Int())
	platform := r.Get("platform").String()
	vendor := r.Get("vendor").String()
	name := r.Get("name").String()
	code := r.Get("code").String()
	style := r.Get("style").String()
	hot := gconv.Int32(r.Get("hot").Int())
	recommend := gconv.Int32(r.Get("recommend").Int())
	isNew := gconv.Int32(r.Get("is_new").Int())
	status := gconv.Int32(r.Get("status").Int())
	page := gconv.Int32(r.Get("page").Int())
	size := gconv.Int32(r.Get("size").Int())

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	client := v7.NewGameClient(conn)
	req := &v7.GetGameListReq{
		SiteId:    siteId,
		Platform:  platform,
		Vendor:    vendor,
		Name:      name,
		Code:      code,
		Style:     style,
		Hot:       hot,
		Recommend: recommend,
		IsNew:     isNew,
		Status:    status,
		Page:      page,
		Size:      size,
	}

	util.LogWithTrace(ctx, "info", "获取游戏列表请求: site_id=%d, platform=%s, page=%d, size=%d",
		siteId, platform, page, size)

	res, err := client.GetGameList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGameList failed: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "获取游戏列表成功: total=%d, page=%d, size=%d",
		res.Total, res.Page, res.Size)

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏资源管理
 * @title 更新游戏资源
 * @description 更新game_list表中的游戏资源信息（只更新后台管理字段）
 * @method post
 * @url /api/backend/game/update-game-list-item
 * @param id 必选 int 游戏ID
 * @param site_id 可选 int 站点ID
 * @param alias_name 可选 string 游戏别名（后台自定义）
 * @param hot 可选 int 是否热门（-1=不修改 0=否 1=是）
 * @param recommend 可选 int 是否推荐（-1=不修改 0=否 1=是）
 * @param is_new 可选 int 是否新游戏（-1=不修改 0=否 1=是）
 * @param status 可选 int 状态（-1=不修改 0=禁用 1=启用）
 * @param sort 可选 int 排序（-1=不修改，数值越大越靠前）
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"更新成功"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 返回消息
 * @remark 注意：此接口只更新后台管理字段（alias_name、hot、recommend、is_new、status、sort），不会更新三方同步字段（name、img、vendor等）
 * @number 8
 */
func CallUpdateBackendGameListItem(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game UpdateGameListItem")

	reqData, err := middleware.GetRequestDataWithFallback(ctx, r)
	if err != nil {
		util.WriteBadRequest(r, "请求数据解析失败")
		return err
	}

	id := gconv.Int32(reqData["id"])
	if id <= 0 {
		util.WriteBadRequest(r, "游戏ID不能为空")
		return fmt.Errorf("id is required")
	}

	siteId := gconv.Int32(reqData["site_id"])
	aliasName := gconv.String(reqData["alias_name"])
	hot := gconv.Int32(reqData["hot"])
	recommend := gconv.Int32(reqData["recommend"])
	isNew := gconv.Int32(reqData["is_new"])
	status := gconv.Int32(reqData["status"])
	sort := gconv.Int32(reqData["sort"])

	client := v7.NewGameClient(conn)
	req := &v7.UpdateGameListItemReq{
		SiteId:    siteId,
		Id:        id,
		AliasName: aliasName,
		Hot:       hot,
		Recommend: recommend,
		IsNew:     isNew,
		Status:    status,
		Sort:      sort,
	}

	util.LogWithTrace(ctx, "info", "更新游戏列表项请求: id=%d, site_id=%d, hot=%d, recommend=%d",
		id, siteId, hot, recommend)

	res, err := client.UpdateGameListItem(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game UpdateGameListItem failed: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "更新游戏列表项完成: success=%v, message=%s",
		res.Success, res.Message)

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/游戏/游戏管理
 * @title 获取游戏种类
 * @description 获取指定站点的游戏种类列表，支持按游戏类型过滤（站点ID从token中获取）
 * @method get
 * @url /api/game/game-categories
 * @param type 可选 int 游戏类型（1=体育，2=彩票，3=真人，4=电子，5=棋牌）
 * @return {"code":0,"msg":"success","data":{"categories":[{"game_id":1,"name":"ONE游戏","platform":"ONE","type":4,"image_code":"one_game","db_name":"one_db","sort":100,"status":1}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.categories array 游戏种类列表
 * @number 5
 */
func CallGetGameCategories(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGameCategories")

	gameType := gconv.Int32(r.Get("type").Int())

	req := &v20.GetGameCategoriesReq{
		Type: gameType,
	}

	client := v20.NewGameClient(conn)
	res, err := client.GetGameCategories(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGameCategories failed: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "获取游戏种类成功: count=%d", len(res.Categories))

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/游戏/游戏管理
 * @title 获取游戏列表
 * @description 获取指定站点的游戏列表，支持按平台、热门、新游戏、推荐等条件过滤（站点ID从token中获取）
 * @method get
 * @url /api/game/game-list
 * @param platform 可选 string 平台（ONE、IG_JILI等）
 * @param hot 可选 int 是否热门（1=是）
 * @param is_new 可选 int 是否新游戏（1=是）
 * @param recommend 可选 int 是否推荐（1=是）
 * @param page 可选 int 页码（默认1）
 * @param size 可选 int 每页数量（默认20，最大100）
 * @return {"code":0,"msg":"success","data":{"list":[{"id":1,"site_id":1,"platform":"ONE","vendor":"PP","code":"vs20olympgate","name":"Gates of Olympus","style":"SLOTS","img":"https://example.com/game.png","img1":"","hot":1,"recommend":1,"is_new":0,"status":1,"sort":100}],"total":1235}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 游戏列表
 * @return_param data.total int 总记录数
 * @number 6
 */
func CallGetFrontendGameList(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGameList (Frontend)")

	platform := r.Get("platform").String()
	hot := gconv.Int32(r.Get("hot").Int())
	isNew := gconv.Int32(r.Get("is_new").Int())
	recommend := gconv.Int32(r.Get("recommend").Int())
	page := gconv.Int32(r.Get("page").Int())
	size := gconv.Int32(r.Get("size").Int())

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	req := &v20.GetGameListReq{
		Platform:  platform,
		Hot:       hot,
		IsNew:     isNew,
		Recommend: recommend,
		Page:      page,
		Size:      size,
	}

	client := v20.NewGameClient(conn)
	res, err := client.GetGameList(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGameList (Frontend) failed: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "获取游戏列表成功: total=%d, page=%d, size=%d",
		res.Total, page, size)

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 后台/游戏/游戏平台
 * @title 获取游戏平台列表
 * @description 获取游戏平台列表，支持按站点ID和游戏类型过滤
 * @method get
 * @url /api/backend/game/game-platforms
 * @param site_id 可选 int 站点ID（0表示不过滤）
 * @param type 可选 int 游戏类型（1=体育，2=彩票，3=真人，4=电子，5=棋牌，0表示不过滤）
 * @return {"code":0,"msg":"success","data":{"platforms":[{"game_id":1,"name":"ONE游戏","platform":"ONE","type":4,"image_code":"one_game","db_name":"one_db","sort":100,"status":1}]}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.platforms array 游戏平台列表
 * @number 9
 */
func CallGetGamePlatforms(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetGamePlatforms")

	siteId := gconv.Int32(r.Get("site_id").Int())
	gameType := gconv.Int32(r.Get("type").Int())

	req := &v7.GetGamePlatformsReq{
		SiteId: siteId,
		Type:   gameType,
	}

	client := v7.NewGameClient(conn)
	res, err := client.GetGamePlatforms(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetGamePlatforms failed: %v", err)
		return err
	}

	util.LogWithTrace(ctx, "info", "获取游戏平台列表成功: count=%d", len(res.Platforms))

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/游戏/游戏管理
 * @title 一键回收游戏余额
 * @description 一键回收三方游戏余额到中心钱包（站点ID和用户ID从token中获取）
 * @method post
 * @url /api/game/recall-balance
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"回收成功","recalled_amount":100.50,"game_id":1,"game_name":"ONE游戏"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.success bool 是否成功
 * @return_param data.message string 返回消息
 * @return_param data.recalled_amount float 回收的金额
 * @return_param data.game_id int 回收的游戏ID
 * @return_param data.game_name string 回收的游戏名称
 * @number 10
 */
func CallRecallGameBalance(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game RecallGameBalance")

	req := &v20.RecallGameBalanceReq{}

	client := v20.NewGameClient(conn)
	res, err := client.RecallGameBalance(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game RecallGameBalance failed: %v", err)
		util.WriteInternalError(r, "回收游戏余额失败，请稍后重试")
		return nil
	}

	util.LogWithTrace(ctx, "info", "一键回收游戏余额完成: success=%v, message=%s, amount=%.2f, game_id=%d",
		res.Success, res.Message, res.RecalledAmount, res.GameId)

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/游戏/游戏记录
 * @title 获取历史投注列表
 * @description 获取当前用户的历史投注列表（按时间倒序）
 * @method get
 * @url /api/game/bet-history
 * @param start_time 可选 string 开始时间（默认当天 00:00:00）
 * @param end_time 可选 string 结束时间（默认当天 23:59:59）
 * @param page 可选 int 页码（默认1）
 * @param size 可选 int 每页数量（默认5，最大100）
 * @return {"code":0,"msg":"success","data":{"list":[{"game_name":"XXX","valid_amount":10.23,"bet_time":"2026-02-03 12:00:00","win_amount":5.12,"status":1,"order_no":"ORD123","net_amount":-5.11}],"total":1}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @return_param data.list array 列表
 * @return_param data.total int 总数
 * @number 11
 */
func CallGetUserBetHistory(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetUserBetHistory (Frontend)")

	startTime := r.Get("start_time").String()
	endTime := r.Get("end_time").String()
	page := gconv.Int32(r.Get("page").Int())
	size := gconv.Int32(r.Get("size").Int())

	req := &v20.GetUserBetHistoryReq{
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		Size:      size,
	}

	client := v20.NewGameClient(conn)
	res, err := client.GetUserBetHistory(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetUserBetHistory failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}

/**
 * showdoc
 * @catalog 前台/游戏/洗码
 * @title 获取可洗码额度
 * @description 获取当前用户的可洗码有效投注额度
 * @method get
 * @url /api/frontend/game/wash-code-quota
 * @param last_bet_id 可选 uint64 上次洗码游标ID（不传则自动按最近洗码时间统计）
 * @return {"code":0,"msg":"success","data":{"success":true,"message":"查询成功","valid_bet_amount":100.0,"max_bet_id":12345,"last_rebate_time":"2026-02-07 12:00:00"}}
 * @return_param code int 状态码
 * @return_param msg string 返回信息
 * @return_param data object 返回数据
 * @number 12
 */
func CallGetWashCodeQuota(ctx context.Context, conn *grpc.ClientConn, r *ghttp.Request) error {
	util.LogWithTrace(ctx, "info", "calling gRPC Game GetWashCodeQuota (Frontend)")

	lastBetId := gconv.Uint64(r.Get("last_bet_id").String())

	req := &v20.GetWashCodeQuotaReq{
		LastBetId: lastBetId,
	}

	client := v20.NewGameClient(conn)
	res, err := client.GetWashCodeQuota(ctx, req)
	if err != nil {
		util.LogWithTrace(ctx, "error", "gRPC Game GetWashCodeQuota failed: %v", err)
		return err
	}

	return writeProtobufResponse(ctx, r, res)
}
