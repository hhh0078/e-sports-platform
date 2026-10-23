// Package app 持久保存公开分享目标并实时生成脱敏元数据，二维码映射不依赖临时缓存。
package app

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"jijie/internal/assets"
)

const shareEnvironment = "release"

var shareIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// ShareRecord 只保存永久目标及分享人引用，不保存登录秘密或未公开业务内容。
type ShareRecord struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	TargetID  string `json:"target_id"`
	GameID    string `json:"game_id"`
	SharerID  string `json:"sharer_id"`
	CreatedAt string `json:"created_at"`
}

// SharePerson 为海报公开展示的分享人，不含角色、手机号、OpenID或会话。
type SharePerson struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

// ShareMetadata 返回实时公开内容和分离的微信页面路径、查询参数及真实小程序码地址。
type ShareMetadata struct {
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	TargetID    string      `json:"target_id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	GameID      string      `json:"game_id"`
	CoverURL    string      `json:"cover_url"`
	Sharer      SharePerson `json:"sharer"`
	Path        string      `json:"path"`
	Query       string      `json:"query"`
	CodeURL     string      `json:"code_url"`
	EnvVersion  string      `json:"env_version"`
	AmountCents int64       `json:"amount_cents"`
	Unit        string      `json:"unit"`
}

// registerShareRoutes 注册公开需求、登录建分享及匿名解析和图片入口。
func (a *App) registerShareRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/shares", a.wrap(a.createShare))
	mux.HandleFunc("GET /api/shares/{id}", a.wrap(a.readShare))
	mux.HandleFunc("GET /api/shares/{id}/code", a.shareCodeHTTP)
	mux.HandleFunc("GET /api/jobs/{id}", a.wrap(a.publicJob))
}

// createShare 仅从已验证会话确定分享人，客户端不能冒充别人或指定任意跳转地址。
func (a *App) createShare(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Kind     string `json:"kind"`
		TargetID string `json:"target_id"`
		GameID   string `json:"game_id"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !contains([]string{"home", "service", "request", "player", "club"}, in.Kind) || !length(in.TargetID, 0, 128) || !length(in.GameID, 0, 32) || (in.Kind == "home") != (in.TargetID == "") {
		return nil, fail(400, "分享类型或目标不正确")
	}
	if err = a.throttle(r.Context(), "share-create", u.ID, 20); err != nil {
		return nil, err
	}
	record := ShareRecord{ID: newID(), Kind: in.Kind, TargetID: in.TargetID, GameID: in.GameID, SharerID: u.ID, CreatedAt: now()}
	metadata, err := a.shareMetadata(r.Context(), record)
	if err != nil {
		return nil, err
	}
	// 服务和需求的游戏随目标实时读取，永久分享不会因服务正常换游戏而停留在旧快照。
	if in.Kind == "service" || in.Kind == "request" {
		record.GameID = ""
	} else {
		record.GameID = metadata.GameID
	}
	if err = put(r.Context(), a.DB, "shares", record.ID, record); err != nil {
		return nil, err
	}
	w.Header().Set("Cache-Control", "no-store")
	return metadata, nil
}

// readShare 每次重新校验目标和账号状态，避免下架或冻结后仍通过旧海报泄露内容。
func (a *App) readShare(w http.ResponseWriter, r *http.Request) (any, error) {
	record, err := a.shareRecord(r.Context(), r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	w.Header().Set("Cache-Control", "no-store")
	return a.shareMetadata(r.Context(), record)
}

// shareRecord 只接受固定长度随机分享号，不将URL、用户编号或未编码参数当作分享令牌。
func (a *App) shareRecord(ctx context.Context, id string) (ShareRecord, error) {
	if !shareIDPattern.MatchString(id) {
		return ShareRecord{}, fail(404, "分享不存在或已失效")
	}
	record, err := get[ShareRecord](ctx, a.DB, "shares", id, false)
	if missing(err) {
		return record, fail(404, "分享不存在或已失效")
	}
	return record, err
}

// shareGame 选择已启用游戏，显式传入无效编号时不悄悄切换成其他游戏。
func (a *App) shareGame(ctx context.Context, id string) (Game, error) {
	if id != "" {
		g, err := get[Game](ctx, a.DB, "games", id, false)
		if missing(err) || err == nil && !g.Enabled {
			return g, fail(404, "该游戏暂不可分享")
		}
		return g, err
	}
	games, err := list[Game](ctx, a.DB, "games", "WHERE data->>'enabled'='true' ORDER BY (data->>'sort_order')::int,id LIMIT 1")
	if err != nil {
		return Game{}, err
	}
	if len(games) == 0 {
		return Game{}, fail(404, "暂无可分享的游戏")
	}
	return games[0], nil
}

// shareImage 仅返回真实存在的本站公开图片，外部地址不下载也不带入分享渲染。
func (a *App) shareImage(ctx context.Context, raw string) string {
	if _, err := assets.PublicCoverData(raw); err == nil || assets.IsDefaultAvatar(raw) {
		return raw
	}
	match := publicImagePath.FindStringSubmatch(raw)
	if match == nil {
		return ""
	}
	v, err := get[Evidence](ctx, a.DB, "evidence", match[1], false)
	if err == nil && v.Public && v.OrderID == "" && len(v.Data) > 0 {
		return raw
	}
	return ""
}

// shareDescription 限制海报摘要长度，不改变原始详情内容。
func shareDescription(value string) string {
	text := []rune(strings.TrimSpace(value))
	if len(text) > 120 {
		return string(text[:120]) + "…"
	}
	return string(text)
}

// shareMetadata 只拼装五类公开目标，交易私密字段和管理页参数不会进入分享响应。
func (a *App) shareMetadata(ctx context.Context, record ShareRecord) (ShareMetadata, error) {
	m := ShareMetadata{ID: record.ID, Kind: record.Kind, TargetID: record.TargetID, CodeURL: "/api/shares/" + record.ID + "/code", EnvVersion: shareEnvironment}
	sharer, err := get[User](ctx, a.DB, "users", record.SharerID, false)
	if missing(err) || err == nil && sharer.Status != "active" {
		return m, fail(404, "分享不存在或已失效")
	}
	if err != nil {
		return m, err
	}
	m.Sharer = SharePerson{ID: sharer.ID, Nickname: sharer.Nickname, AvatarURL: a.shareImage(ctx, sharer.Avatar)}
	if m.Sharer.AvatarURL == "" {
		m.Sharer.AvatarURL = stableDefaultAvatar("user", sharer.ID)
	}
	gameID, cover := record.GameID, ""
	query := url.Values{"share_id": {record.ID}}
	switch record.Kind {
	case "home":
		settings, e := get[Settings](ctx, a.DB, "settings", "platform", false)
		if e != nil {
			return m, e
		}
		m.Title, m.Description, m.Path = settings.SiteName+" · 找队友，也成为队友", "游戏爱好者的接单平台，让擅长成为收入", "pages/main/index"
		query.Set("tab", "player")
	case "service":
		s, e := get[Service](ctx, a.DB, "services", record.TargetID, false)
		if missing(e) || e == nil && s.Status != "active" {
			return m, fail(404, "服务不存在或已下架")
		}
		if e != nil {
			return m, e
		}
		provider, e := get[User](ctx, a.DB, "users", s.ProviderID, false)
		if missing(e) || e == nil && provider.Status != "active" {
			return m, fail(404, "服务暂不可分享")
		}
		if e != nil {
			return m, e
		}
		gameID, cover, m.Title, m.Description, m.Path, m.AmountCents, m.Unit = s.GameID, s.CoverURL, s.Title, s.Description, "pages/services/detail", s.PriceCents, s.Unit
		query.Set("id", s.ID)
	case "request":
		job, e := a.sharePublicJob(ctx, record.TargetID)
		if e != nil {
			return m, e
		}
		gameID, m.Title, m.Description, m.Path, m.AmountCents, m.Unit = job.GameID, job.Title, job.Requirements, "pages/requests/detail", job.AmountCents, job.Unit
		query.Set("id", job.ID)
	case "player":
		u, e := get[User](ctx, a.DB, "users", record.TargetID, false)
		if missing(e) || e == nil && u.Status != "active" {
			return m, fail(404, "玩家不存在或暂不可分享")
		}
		if e != nil {
			return m, e
		}
		cover, m.Title, m.Description, m.Path = u.ProfileCover, u.Nickname+"的集结主页", u.Bio, "pages/community/players/detail"
		query.Set("id", u.ID)
	case "club":
		c, e := get[Club](ctx, a.DB, "clubs", record.TargetID, false)
		if missing(e) || e == nil && c.Status != "active" {
			return m, fail(404, "俱乐部不存在或尚未公开")
		}
		if e != nil {
			return m, e
		}
		owner, e := get[User](ctx, a.DB, "users", c.OwnerID, false)
		if missing(e) || e == nil && owner.Status != "active" {
			return m, fail(404, "俱乐部暂不可分享")
		}
		if e != nil {
			return m, e
		}
		if gameID != "" && !contains(c.GameIDs, gameID) {
			return m, fail(400, "所选游戏不在俱乐部范围内")
		}
		if gameID == "" {
			for _, candidate := range c.GameIDs {
				if selected, gameErr := a.shareGame(ctx, candidate); gameErr == nil {
					gameID = selected.ID
					break
				}
			}
			if gameID == "" {
				return m, fail(404, "俱乐部暂无可分享游戏")
			}
		}
		cover, m.Title, m.Description, m.Path = c.CoverURL, c.Name+" · 集结", c.Description, "pages/community/clubs/detail"
		query.Set("id", c.ID)
	default:
		return m, fail(404, "分享类型不存在")
	}
	if record.GameID != "" && (record.Kind == "service" || record.Kind == "request") && record.GameID != gameID {
		return m, fail(400, "分享游戏与目标不一致")
	}
	g, err := a.shareGame(ctx, gameID)
	if err != nil {
		return m, err
	}
	m.GameID = g.ID
	m.CoverURL = a.shareImage(ctx, cover)
	if m.CoverURL == "" {
		m.CoverURL = a.shareImage(ctx, g.CoverURL)
	}
	if m.CoverURL == "" {
		return m, fail(409, "分享封面尚未托管，请联系管理员上传游戏封面")
	}
	if record.Kind == "home" {
		query.Set("game_id", g.ID)
	}
	m.Description, m.Query = shareDescription(m.Description), query.Encode()
	return m, nil
}

// sharePublicJob 仅允许读取已付款待接单公开需求，不暴露买家标识、原始约定或聊天。
func (a *App) sharePublicJob(ctx context.Context, id string) (PublicJob, error) {
	o, err := get[Order](ctx, a.DB, "orders", id, false)
	if missing(err) || err == nil && (o.ServiceID != "" || o.Status != "open" || o.ProviderID != "" || o.PaymentStatus != "paid" || contains([]string{"requested", "escalated", "processing"}, o.RefundStatus)) {
		return PublicJob{}, fail(404, "需求不存在或已结束招募")
	}
	if err != nil {
		return PublicJob{}, err
	}
	buyer, err := get[User](ctx, a.DB, "users", o.BuyerID, false)
	if missing(err) || err == nil && buyer.Status != "active" {
		return PublicJob{}, fail(404, "需求暂不可查看")
	}
	if err != nil {
		return PublicJob{}, err
	}
	if _, err = a.shareGame(ctx, o.GameID); err != nil {
		return PublicJob{}, err
	}
	return PublicJob{ID: o.ID, GameID: o.GameID, GameName: o.GameName, Title: o.Title, Quantity: o.Quantity, AmountCents: o.AmountCents, Status: o.Status, PaymentStatus: o.PaymentStatus, Requirements: "接单后查看完整需求并与老板沟通", CreatedAt: o.CreatedAt, Unit: o.Unit, UnitPriceCents: o.UnitPriceCents}, nil
}

// publicJob 返回与公开大厅一致的脱敏需求，完整订单权限保持原样。
func (a *App) publicJob(w http.ResponseWriter, r *http.Request) (any, error) {
	w.Header().Set("Cache-Control", "no-store")
	return a.sharePublicJob(r.Context(), r.PathValue("id"))
}
