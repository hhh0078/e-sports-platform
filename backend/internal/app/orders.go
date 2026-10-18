// Package app 处理订单创建与参与者视图，金额和身份始终由服务端确定。
package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// baseOrder 创建未付款订单及唯一流水号。
func baseOrder(u User) Order {
	id := newID()
	t := now()
	return Order{ID: id, OrderNo: "JJ" + strings.ReplaceAll(t[:10], "-", "") + strings.ToUpper(id[:10]), BuyerID: u.ID, BuyerName: u.Nickname, Quantity: 1, Status: "pending_payment", PaymentStatus: "unpaid", RefundStatus: "none", CreatedAt: t, UpdatedAt: t}
}

// systemMessage 将交易关键事件追加为不可由用户伪造的系统证据。
func systemMessage(ctx context.Context, q querier, o Order, content string) error {
	m := Message{ID: newID(), OrderID: o.ID, SenderID: "system", SenderName: "集结平台", SenderRole: "system", Kind: "system", Content: content, CreatedAt: now()}
	return put(ctx, q, "messages", m.ID, m)
}

// createOrder 在事务中快照商品价格、接单人和俱乐部归属。
func (a *App) createOrder(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		ServiceID    string `json:"service_id"`
		Quantity     int    `json:"quantity"`
		Requirements string `json:"requirements"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if in.Quantity < 1 || in.Quantity > 100 || !length(in.Requirements, 0, 1000) {
		return nil, fail(400, "数量范围 1 至 100，需求最多 1000 字")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	s, err := get[Service](r.Context(), tx, "services", in.ServiceID, true)
	if err != nil {
		return nil, err
	}
	if s.Status != "active" {
		return nil, fail(409, "服务当前不可下单")
	}
	if s.ProviderID == u.ID {
		return nil, fail(400, "不能给自己的服务下单")
	}
	provider, e := get[User](r.Context(), tx, "users", s.ProviderID, false)
	if e != nil || provider.Status != "active" || !provider.ProviderEnabled {
		return nil, fail(409, "打手当前暂不接单")
	}
	g, e := get[Game](r.Context(), tx, "games", s.GameID, false)
	if e != nil || !g.Enabled {
		return nil, fail(409, "游戏当前不可用")
	}
	o := baseOrder(u)
	o.GameID = s.GameID
	o.GameName = g.Name
	o.ServiceID = s.ID
	o.ProviderID = s.ProviderID
	o.ProviderName = provider.Nickname
	o.Title = s.Title
	o.ServiceDescription = s.Description
	o.Unit = s.Unit
	o.UnitPriceCents = s.PriceCents
	o.Quantity = in.Quantity
	o.AmountCents = s.PriceCents * int64(in.Quantity)
	o.Requirements = strings.TrimSpace(in.Requirements)
	if s.ClubID != "" {
		c, ce := get[Club](r.Context(), tx, "clubs", s.ClubID, false)
		m, me := get[ClubMember](r.Context(), tx, "members", s.ClubID+":"+s.ProviderID, false)
		if ce == nil && c.Status == "active" && contains(c.GameIDs, s.GameID) && (c.OwnerID == s.ProviderID || (me == nil && m.Status == "active")) {
			o.ClubID = s.ClubID
		}
	}
	if o.AmountCents > 10000000 {
		return nil, fail(400, "订单总金额不能超过 100000 元")
	}
	if err = put(r.Context(), tx, "orders", o.ID, o); err != nil {
		return nil, err
	}
	if err = systemMessage(r.Context(), tx, o, "订单已创建，等待买家付款。"); err != nil {
		return nil, err
	}
	return o, tx.Commit(r.Context())
}

// createRequest 创建公开需求，支付前不会出现在接单大厅。
func (a *App) createRequest(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		GameID       string `json:"game_id"`
		Title        string `json:"title"`
		Requirements string `json:"requirements"`
		AmountCents  int64  `json:"amount_cents"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !length(in.Title, 2, 40) || !length(in.Requirements, 5, 1000) || in.AmountCents < 100 || in.AmountCents > 10000000 {
		return nil, fail(400, "需求标题 2 至 40 字、说明 5 至 1000 字，金额 1 至 100000 元")
	}
	g, err := get[Game](r.Context(), a.DB, "games", in.GameID, false)
	if err != nil || !g.Enabled {
		return nil, fail(400, "游戏当前不可用")
	}
	o := baseOrder(u)
	o.GameID = g.ID
	o.GameName = g.Name
	o.Title = strings.TrimSpace(in.Title)
	o.Requirements = strings.TrimSpace(in.Requirements)
	o.AmountCents = in.AmountCents
	o.ServiceDescription = in.Requirements
	o.Unit = "次"
	o.UnitPriceCents = in.AmountCents
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = put(r.Context(), tx, "orders", o.ID, o); err != nil {
		return nil, err
	}
	if err = systemMessage(r.Context(), tx, o, "公开需求已创建，付款后进入接单大厅。"); err != nil {
		return nil, err
	}
	return o, tx.Commit(r.Context())
}

// orders 返回用户自身的购买或接单记录，不因管理员身份扩大查询。
func (a *App) orders(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	where := "WHERE (data->>'buyer_id'=$1 OR data->>'provider_id'=$1)"
	switch r.URL.Query().Get("role") {
	case "buyer":
		where = "WHERE data->>'buyer_id'=$1"
	case "provider":
		where = "WHERE data->>'provider_id'=$1"
	case "", "all":
	default:
		return nil, fail(400, "订单视角无效")
	}
	args := []any{u.ID}
	if status := r.URL.Query().Get("status"); status != "" {
		where += " AND data->>'status'=$2"
		args = append(args, status)
	}
	items, err := list[Order](r.Context(), a.DB, "orders", where+" ORDER BY data->>'created_at' DESC", args...)
	return paginate(r, items), err
}

// jobs 公开大厅隐藏具体需求说明、买家标识和聊天证据。
func (a *App) jobs(w http.ResponseWriter, r *http.Request) (any, error) {
	items, err := list[Order](r.Context(), a.DB, "orders", "WHERE data->>'status'='open' AND data->>'provider_id'='' AND data->>'refund_status' NOT IN ('requested','escalated','processing') ORDER BY data->>'created_at' DESC")
	if err != nil {
		return nil, err
	}
	out := []PublicJob{}
	for _, o := range items {
		if g := r.URL.Query().Get("game_id"); g != "" && g != o.GameID {
			continue
		}
		out = append(out, PublicJob{ID: o.ID, GameID: o.GameID, GameName: o.GameName, Title: o.Title, Quantity: o.Quantity, AmountCents: o.AmountCents, Status: o.Status, PaymentStatus: o.PaymentStatus, Requirements: "接单后查看完整需求并与老板沟通", CreatedAt: o.CreatedAt, Unit: o.Unit, UnitPriceCents: o.UnitPriceCents})
	}
	return paginate(r, out), nil
}

// authorizedOrder 校验四方聊天室和订单详情访问权限。
func (a *App) authorizedOrder(r *http.Request, u User, id string, lock bool) (Order, error) {
	o, err := get[Order](r.Context(), a.DB, "orders", id, lock)
	if err != nil {
		return o, err
	}
	if o.BuyerID == u.ID || o.ProviderID == u.ID || u.Role == "admin" {
		return o, nil
	}
	if o.ClubID != "" {
		c, e := get[Club](r.Context(), a.DB, "clubs", o.ClubID, false)
		if e == nil && c.OwnerID == u.ID {
			return o, nil
		}
	}
	return o, fail(403, "无权查看此订单")
}

// order 返回仅参与者可见的完整订单。
func (a *App) order(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	return a.authorizedOrder(r, u, r.PathValue("id"), false)
}

// pay 在本地模式原子记账，重复请求不会再次扣款或重置订单。
func (a *App) pay(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	o, err := get[Order](r.Context(), tx, "orders", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if o.BuyerID != u.ID {
		return nil, fail(403, "只有买家可以付款")
	}
	if o.PaymentStatus == "paid" {
		return map[string]any{"order": o, "mode": paymentMode(a.Config.DevMode)}, nil
	}
	if o.Status != "pending_payment" || o.PaymentStatus != "unpaid" {
		return nil, fail(409, "当前订单无法付款")
	}

	if o.ServiceID != "" {
		s, e := get[Service](r.Context(), tx, "services", o.ServiceID, false)
		p, pe := get[User](r.Context(), tx, "users", o.ProviderID, false)
		if e != nil || s.Status != "active" || pe != nil || p.Status != "active" || !p.ProviderEnabled {
			return nil, fail(409, "服务已暂停，请取消订单")
		}
	}
	if !a.Config.DevMode {
		if _, e := a.loadWXPay(); e != nil {
			return nil, e
		}
		identity, e := get[map[string]string](r.Context(), tx, "identities", u.ID, false)
		if e != nil || identity["openid"] == "" {
			return nil, fail(503, "请先通过微信登录后使用微信支付")
		}
		o.PayOpenID = identity["openid"]
		if err = put(r.Context(), tx, "orders", o.ID, o); err != nil {
			return nil, err
		}
		result, payErr := a.prepay(r, u, o)
		// 即使客户端断开或上游超时，也保存预支付意图供关单和资金查询补偿。
		commitCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()
		if err = tx.Commit(commitCtx); err != nil {
			return nil, err
		}
		if payErr != nil {
			return nil, payErr
		}
		return result, nil
	}
	o.Status = "open"
	o.PaymentStatus = "paid"
	o.UpdatedAt = now()
	if err = put(r.Context(), tx, "orders", o.ID, o); err != nil {
		return nil, err
	}
	if err = systemMessage(r.Context(), tx, o, fmt.Sprintf("测试付款成功，金额 %.2f 元，等待打手接单。本地测试不产生真实扣费。", float64(o.AmountCents)/100)); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return map[string]any{"order": o, "mode": "dev"}, nil
}

// paymentMode 返回显式付款环境标识。
func paymentMode(dev bool) string {
	if dev {
		return "dev"
	}
	return "wechat"
}
