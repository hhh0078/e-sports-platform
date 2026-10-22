// Package app 实现游戏主题、服务列表、发布审核与动态俱乐部认证展示。
package app

import (
	"context"
	"net/http"
	"sort"
	"strings"
)

// bootstrap 提供客户端初始化数据和开发模式提示。
func (a *App) bootstrap(w http.ResponseWriter, r *http.Request) (any, error) {
	games, err := a.games(w, r)
	if err != nil {
		return nil, err
	}
	s, err := get[Settings](r.Context(), a.DB, "settings", "platform", false)
	if err != nil {
		return nil, err
	}
	mode := "unconfigured"
	if cfg, e := a.readAISettings(r.Context(), a.DB); e == nil {
		if cfg.APIKey != "" {
			mode = "ai"
		} else if a.Config.DevMode {
			mode = "local_simulation"
		}
	}
	return map[string]any{"games": games, "moderation_mode": mode, "dev_mode": a.Config.DevMode, "site_name": s.SiteName, "announcement": s.Announcement, "support_qr_url": s.SupportQRURL, "support_phone": s.SupportPhone, "support_wechat": s.SupportWechat}, nil
}

// games 返回已启用的游戏主题。
func (a *App) games(w http.ResponseWriter, r *http.Request) (any, error) {
	items, err := list[Game](r.Context(), a.DB, "games", "WHERE data->>'enabled'='true'")
	sort.Slice(items, func(i, j int) bool {
		if items[i].SortOrder == items[j].SortOrder {
			return items[i].ID < items[j].ID
		}
		return items[i].SortOrder < items[j].SortOrder
	})
	return items, err
}

// decorateService 实时加载昵称和认证，退出俱乐部立即撤销认证显示。
func (a *App) decorateService(ctx context.Context, s Service) Service {
	if u, err := get[User](ctx, a.DB, "users", s.ProviderID, false); err == nil {
		s.ProviderName = u.Nickname
		s.ProviderAvatar = u.Avatar
	}
	s.ProviderDepositCents = 0
	if wallet, err := get[Wallet](ctx, a.DB, "wallets", s.ProviderID, false); err == nil {
		s.ProviderDepositCents = wallet.DepositCents
	}
	s.ProviderClubDepositCents = 0
	s.ClubVerified = false
	s.ClubName = ""
	if s.ClubID != "" {
		c, err := get[Club](ctx, a.DB, "clubs", s.ClubID, false)
		if err == nil && c.Status == "active" && contains(c.GameIDs, s.GameID) {
			s.ClubName = c.Name
			m, e := get[ClubMember](ctx, a.DB, "members", s.ClubID+":"+s.ProviderID, false)
			s.ClubVerified = c.Verified && (c.OwnerID == s.ProviderID || (e == nil && m.Status == "active"))
			if e == nil && m.Status == "active" {
				if wallet, e := accountBalance(ctx, a.DB, memberDepositAccount(s.ClubID, s.ProviderID)); e == nil {
					s.ProviderClubDepositCents = wallet.DepositCents
				}
			}
		}
	}
	return s
}

// services 过滤公开服务或当前用户自己的发布记录。
func (a *App) services(w http.ResponseWriter, r *http.Request) (any, error) {
	ownerID := ""
	if r.URL.Query().Get("mine") == "1" {
		u, err := a.actor(r)
		if err != nil {
			return nil, err
		}
		ownerID = u.ID
	}
	where, args, err := serviceQuery(r, ownerID)
	if err != nil {
		return nil, err
	}
	items, err := list[Service](r.Context(), a.DB, "services", where, args...)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i] = a.decorateService(r.Context(), items[i])
	}
	return items, nil
}

// service 仅向发布者或管理员显示尚未上架的服务。
func (a *App) service(w http.ResponseWriter, r *http.Request) (any, error) {
	s, err := get[Service](r.Context(), a.DB, "services", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	g, ge := get[Game](r.Context(), a.DB, "games", s.GameID, false)
	provider, pe := get[User](r.Context(), a.DB, "users", s.ProviderID, false)
	if s.Status != "active" || ge != nil || !g.Enabled || pe != nil || provider.Status != "active" {
		u, e := a.actor(r)
		if e != nil || (u.ID != s.ProviderID && u.Role != "admin") {
			return nil, fail(404, "服务不存在或尚未上架")
		}
	}
	s.Reviews, err = list[Review](r.Context(), a.DB, "reviews", "WHERE EXISTS(SELECT 1 FROM orders o WHERE o.id=reviews.data->>'order_id' AND o.data->>'service_id'=$1 AND o.data->>'status'='completed' AND o.data->>'refund_status' NOT IN ('requested','escalated','processing')) ORDER BY data->>'created_at' DESC LIMIT 100", s.ID)
	if err != nil {
		return nil, err
	}
	if s.Reviews == nil {
		s.Reviews = []Review{}
	}
	return a.decorateService(r.Context(), s), nil
}

// saveService 发布或编辑服务，所有修改都重新进入审核。
func (a *App) saveService(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if err = a.requireVerifiedPhone(r, u); err != nil {
		return nil, err
	}
	var in struct {
		GameID      string       `json:"game_id"`
		Title       string       `json:"title"`
		Description string       `json:"description"`
		Category    string       `json:"category"`
		PriceCents  int64        `json:"price_cents"`
		Unit        string       `json:"unit"`
		CoverURL    string       `json:"cover_url"`
		Labels      []CoverLabel `json:"labels"`
		Tags        []string     `json:"tags"`
		ClubID      string       `json:"club_id"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if err = a.moderateServiceDraft(r, u, Service{GameID: in.GameID, Title: in.Title, Description: in.Description, Category: in.Category, PriceCents: in.PriceCents, Unit: in.Unit, CoverURL: in.CoverURL, Labels: in.Labels, Tags: in.Tags}); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	s := Service{ID: newID(), ProviderID: u.ID, ProviderName: u.Nickname, CreatedAt: now()}
	if id := r.PathValue("id"); id != "" {
		s, err = get[Service](r.Context(), tx, "services", id, true)
		if err != nil {
			return nil, err
		}
		if s.ProviderID != u.ID && u.Role != "admin" {
			return nil, fail(403, "只能编辑自己的服务")
		}
	}
	provider, err := get[User](r.Context(), tx, "users", s.ProviderID, false)
	if err != nil {
		return nil, err
	}
	if !length(provider.Bio, 2, 300) {
		return nil, fail(400, "请先完善个人简介")
	}
	g, err := get[Game](r.Context(), tx, "games", in.GameID, false)
	if err != nil || !g.Enabled {
		return nil, fail(400, "游戏当前不可用")
	}
	s.GameID = in.GameID
	s.Title = strings.TrimSpace(in.Title)
	s.Description = strings.TrimSpace(in.Description)
	s.Category = in.Category
	s.PriceCents = in.PriceCents
	s.Unit = in.Unit
	s.CoverURL = in.CoverURL
	s.Labels = in.Labels
	s.Tags = in.Tags
	s.ClubID = in.ClubID
	s.Status = "pending"
	if err = validateService(&s, g); err != nil {
		return nil, err
	}
	if s.ClubID != "" {
		c, e := get[Club](r.Context(), tx, "clubs", s.ClubID, false)
		m, me := get[ClubMember](r.Context(), tx, "members", s.ClubID+":"+s.ProviderID, false)
		if e != nil || c.Status != "active" || !contains(c.GameIDs, g.ID) || (c.OwnerID != s.ProviderID && (me != nil || m.Status != "active")) {
			return nil, fail(403, "只能关联本人已通过挂靠且支持该游戏的俱乐部")
		}
	}
	if err = put(r.Context(), tx, "services", s.ID, s); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "service.edit", s.ID, ""); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return a.decorateService(r.Context(), s), nil
}

// pauseService 允许所有者停止新交易，历史订单继续履约。
func (a *App) pauseService(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	s, err := get[Service](r.Context(), tx, "services", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if s.ProviderID != u.ID && u.Role != "admin" {
		return nil, fail(403, "只能暂停自己的服务")
	}
	s.Status = "paused"
	if err = put(r.Context(), tx, "services", s.ID, s); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "service.pause", s.ID, ""); err != nil {
		return nil, err
	}
	return s, tx.Commit(r.Context())
}

// audit 在业务事务中记录不可由用户伪造的操作日志。
func audit(ctx context.Context, q querier, actor, action, target, note string) error {
	v := Audit{ID: newID(), ActorID: actor, Action: action, TargetID: target, Note: note, CreatedAt: now()}
	return put(ctx, q, "audit", v.ID, v)
}
