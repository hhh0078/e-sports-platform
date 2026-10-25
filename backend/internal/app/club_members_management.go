// Package app 为俱乐部管理者提供成员资料、组织内收入和托管押金视图，严格排除个人充值与组织外资金。
package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// managedMemberTarget 同时核验当前组织权限和真实成员绑定，路径中更换任一ID都不能越权。
func (a *App) managedMemberTarget(r *http.Request) (User, Club, ClubMember, User, error) {
	u, err := a.actor(r)
	if err != nil {
		return u, Club{}, ClubMember{}, User{}, err
	}
	c, err := a.clubManager(r, u, r.PathValue("id"))
	if err != nil {
		return u, c, ClubMember{}, User{}, err
	}
	id := r.PathValue("user_id")
	m, err := get[ClubMember](r.Context(), a.DB, "members", c.ID+":"+id, false)
	if missing(err) && id == c.OwnerID {
		m = ClubMember{ClubID: c.ID, UserID: id, Status: "active", CreatedAt: c.CreatedAt}
		err = nil
	}
	if err != nil || m.ClubID != c.ID || m.UserID != id {
		return u, c, m, User{}, fail(404, "成员不属于此俱乐部")
	}
	member, err := get[User](r.Context(), a.DB, "users", id, false)
	return u, c, m, member, err
}

// managedMembers 在当前组织范围内按用户昵称、ID或手机号先检索再分页，不返回完整手机号和简介。
func (a *App) managedMembers(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	c, err := a.clubManager(r, u, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	where, args := "data->>'club_id'=$1", []any{c.ID}
	if status := r.URL.Query().Get("status"); status != "" && status != "all" {
		if !contains([]string{"pending", "active", "rejected", "removed", "left"}, status) {
			return nil, fail(400, "成员状态无效")
		}
		args = append(args, status)
		where += fmt.Sprintf(" AND data->>'status'=$%d", len(args))
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if !length(query, 0, 100) {
		return nil, fail(400, "搜索关键词最多100字")
	}
	if query != "" {
		args = append(args, query)
		p := fmt.Sprintf("$%d", len(args))
		where += " AND EXISTS(SELECT 1 FROM users member_user LEFT JOIN identities member_identity ON member_identity.id=member_user.id WHERE member_user.id=members.data->>'user_id' AND strpos(lower(concat_ws(' ',member_user.id,member_user.data->>'nickname',member_user.data->>'phone_masked',member_identity.data->>'phone_number')),lower(" + p + "))>0)"
	}
	page, err := managedPage[ClubMember](r.Context(), a.DB, "members", where, args, r)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for _, member := range page.Items {
		user, e := get[User](r.Context(), a.DB, "users", member.UserID, false)
		if e != nil {
			return nil, e
		}
		role, e := clubManagementRole(r.Context(), a.DB, user, c)
		if e != nil {
			return nil, e
		}
		v := a.memberFinancialView(r.Context(), member, user, true)
		v["phone_masked"] = user.PhoneMasked
		v["management_role"] = role
		items = append(items, v)
	}
	return ManagementPage[map[string]any]{items, page.Total, page.Limit, page.Offset, page.HasMore}, nil
}

// ScopedMemberFinance 只表示本组织已结算累计收入、在途款和成员托管押金，不推测个人可提现余额。
type ScopedMemberFinance struct {
	ClubID                    string `json:"club_id"`
	UserID                    string `json:"user_id"`
	SettledIncomeCents        int64  `json:"settled_income_cents"`
	OngoingCents              int64  `json:"ongoing_cents"`
	SettlingCents             int64  `json:"settling_cents"`
	ClubDepositCents          int64  `json:"club_deposit_cents"`
	ClubDepositRefundingCents int64  `json:"club_deposit_refunding_cents"`
}

// memberIncomePredicate 只识别提供者收入分录及其原订单／调价归属，不包含买方消费、充值和提现。
func memberIncomePredicate() string {
	return `(data->>'user_id'=$1 AND data->>'kind' IN ('order_pending','order_settlement','order_refund_reversal','order_price_adjustment','order_adjusted_full_refund') AND (EXISTS(SELECT 1 FROM orders income_order WHERE income_order.id=wallet_entries.data->>'reference_id' AND income_order.data->>'club_id'=$2 AND income_order.data->>'provider_id'=$1) OR EXISTS(SELECT 1 FROM order_adjustments adjustment JOIN orders income_order ON income_order.id=adjustment.data->>'order_id' WHERE adjustment.id=wallet_entries.data->>'reference_id' AND income_order.data->>'club_id'=$2 AND income_order.data->>'provider_id'=$1)))`
}

// scopedMemberFinance 从实际资金分录与托管状态汇总，不因订单展示状态凭空增加历史收入。
func (a *App) scopedMemberFinance(ctx context.Context, club, user string) (ScopedMemberFinance, error) {
	out := ScopedMemberFinance{ClubID: club, UserID: user}
	deposit, err := accountBalance(ctx, a.DB, memberDepositAccount(club, user))
	if err != nil {
		return out, err
	}
	out.ClubDepositCents = deposit.DepositCents
	out.ClubDepositRefundingCents = deposit.DepositRefundingCents
	err = a.DB.QueryRow(ctx, "SELECT COALESCE(sum((data->>'available_delta')::bigint),0) FROM wallet_entries WHERE "+memberIncomePredicate()+" AND data->>'kind'='order_settlement'", user, club).Scan(&out.SettledIncomeCents)
	if err != nil {
		return out, err
	}
	orders, err := list[Order](ctx, a.DB, "orders", "WHERE data->>'club_id'=$1 AND data->>'provider_id'=$2 AND EXISTS(SELECT 1 FROM wallet_escrows e WHERE e.id=orders.id AND e.data->>'provider_id'=$2 AND e.data->>'status'='holding')", club, user)
	if err != nil {
		return out, err
	}
	for _, order := range orders {
		amount := settlementForOrder(order).ProviderIncomeCents
		if order.Status == "completed" {
			out.SettlingCents += amount
		} else {
			out.OngoingCents += amount
		}
	}
	return out, nil
}

// managedMemberDetail 返回组织成员局部管理资料，用户个人钱包与其它组织收入永远不进入响应。
func (a *App) managedMemberDetail(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, c, m, user, err := a.managedMemberTarget(r)
	if err != nil {
		return nil, err
	}
	role, err := clubManagementRole(r.Context(), a.DB, user, c)
	if err != nil {
		return nil, err
	}
	finance, err := a.scopedMemberFinance(r.Context(), c.ID, user.ID)
	if err != nil {
		return nil, err
	}
	canGrant := canManageClubAdmins(actor, c) && user.ID != c.OwnerID
	return map[string]any{"user": managementUserSummary(user), "membership": m, "management_role": role, "permissions": ManagementPermissions{user.ID != c.OwnerID && (canManageClubAdmins(actor, c) || role != "deputy"), canGrant, true}, "scope_finance": finance}, nil
}

// managedMemberMessage 使用已验证的组织成员路径发送定向站内通知。
func (a *App) managedMemberMessage(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, c, _, user, err := a.managedMemberTarget(r)
	if err != nil {
		return nil, err
	}
	return a.sendManagedUserMessage(r, actor, user.ID, c.ID)
}

// managedMemberOrders 只返回该成员作为打手在当前组织提供服务的订单。
func (a *App) managedMemberOrders(w http.ResponseWriter, r *http.Request) (any, error) {
	_, c, _, u, err := a.managedMemberTarget(r)
	if err != nil {
		return nil, err
	}
	where, args, err := managedOrderStatus(r, "data->>'club_id'=$1 AND data->>'provider_id'=$2", []any{c.ID, u.ID})
	if err != nil {
		return nil, err
	}
	page, err := managedPage[Order](r.Context(), a.DB, "orders", where, args, r)
	page.Items = publicValue(page.Items).([]Order)
	return page, err
}

// managedMemberEntries 返回本组织收入及独立成员押金的流水，提款与充值不能通过关联记录泄露。
func (a *App) managedMemberEntries(w http.ResponseWriter, r *http.Request) (any, error) {
	_, c, _, u, err := a.managedMemberTarget(r)
	if err != nil {
		return nil, err
	}
	account := memberDepositAccount(c.ID, u.ID)
	page, err := managedPage[WalletEntry](r.Context(), a.DB, "wallet_entries", "("+memberIncomePredicate()+" OR data->>'user_id'=$3)", []any{u.ID, c.ID, account}, r)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for _, entry := range page.Items {
		scope := "club_order_income"
		orderID := ""
		if entry.UserID == account {
			scope = "club_member_deposit"
		} else {
			e := a.DB.QueryRow(r.Context(), "SELECT id FROM orders WHERE data->>'club_id'=$1 AND data->>'provider_id'=$2 AND (id=$3 OR id=(SELECT data->>'order_id' FROM order_adjustments WHERE id=$3))", c.ID, u.ID, entry.ReferenceID).Scan(&orderID)
			if e != nil {
				return nil, e
			}
		}
		items = append(items, map[string]any{"id": entry.ID, "user_id": u.ID, "kind": entry.Kind, "reference_id": entry.ReferenceID, "amount_cents": entry.AmountCents, "available_delta": entry.AvailableDelta, "pending_delta": entry.PendingDelta, "deposit_delta": entry.DepositDelta, "deposit_refunding_delta": entry.DepositRefundingDelta, "recharge_delta": 0, "withdrawal_frozen_delta": 0, "note": entry.Note, "created_at": entry.CreatedAt, "scope": scope, "order_id": orderID})
	}
	return ManagementPage[map[string]any]{items, page.Total, page.Limit, page.Offset, page.HasMore}, nil
}

// managedMemberModeration 返回本组织与该成员绑定的审核摘要，不返回原交易证据或其它组织资料。
func (a *App) managedMemberModeration(w http.ResponseWriter, r *http.Request) (any, error) {
	_, c, _, u, err := a.managedMemberTarget(r)
	if err != nil {
		return nil, err
	}
	return a.managementModerationPage(r, "((data->>'club_id'=$1 AND data->>'owner_id'=$2) OR (data->>'target_type'='members' AND data->>'target_id'=$3))", []any{c.ID, u.ID, c.ID + ":" + u.ID})
}
