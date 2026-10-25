// Package app 隔离成员俱乐部押金、组织平台押金与俱乐部佣金，付款和原路退款复用已核验的微信资金状态机。
package app

import (
	"context"
	"net/http"
	"time"
)

// clubDepositAccount 返回组织平台押金专户，绝不作为佣金可提余额。
func clubDepositAccount(id string) string { return "club-deposit:" + id }

// memberDepositAccount 返回用户与组织绑定的托管押金专户。
func memberDepositAccount(club, user string) string { return "member-deposit:" + club + ":" + user }

// accountBalance 读取不存在时为零的资金专户。
func accountBalance(ctx context.Context, q querier, id string) (Wallet, error) {
	v, e := get[Wallet](ctx, q, "wallets", id, false)
	if missing(e) {
		return Wallet{UserID: id}, nil
	}
	return v, e
}

// lockClubFinance 在统一用户锁之前取得组织锁，组织退款、认证及新挂靠订单共享此锁。
func lockClubFinance(ctx context.Context, q querier, id string) error {
	if id == "" {
		return nil
	}
	_, e := q.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "jijie-club-finance:"+id)
	return e
}

// guardClubDeposit 拒绝退还中或未补足平台押金的组织新增认证及挂靠交易。
func guardClubDeposit(ctx context.Context, q querier, id string) error {
	if id == "" {
		return nil
	}
	if e := lockClubFinance(ctx, q, id); e != nil {
		return e
	}
	settings, e := financialSettings(ctx, q)
	if e != nil {
		return e
	}
	wallet, e := accountBalance(ctx, q, clubDepositAccount(id))
	if e != nil {
		return e
	}
	if wallet.DepositRefundingCents > 0 {
		return fail(409, "俱乐部平台押金正在退还，暂不可新增认证或挂靠订单")
	}
	club, e := get[Club](ctx, q, "clubs", id, false)
	if e != nil {
		return e
	}
	if wallet.DepositCents < clubRequiredDeposit(club, settings) {
		return fail(409, "俱乐部尚未补足平台押金，请负责人先缴纳")
	}
	return nil
}

// depositAccount 根据服务端权限确定押金专户，客户端不能指定任意账户。
func (a *App) depositAccount(ctx context.Context, q querier, u User, purpose, clubID string) (string, error) {
	if purpose == "deposit" || purpose == "balance" {
		if clubID != "" {
			return "", fail(400, "个人资金不能指定俱乐部")
		}
		return u.ID, nil
	}
	if !contains([]string{"club_platform", "member_club"}, purpose) || clubID == "" {
		return "", fail(400, "押金用途或俱乐部无效")
	}
	if e := lockClubFinance(ctx, q, clubID); e != nil {
		return "", e
	}
	c, e := get[Club](ctx, q, "clubs", clubID, false)
	if e != nil {
		return "", e
	}
	if purpose == "club_platform" {
		role, err := clubManagementRole(ctx, q, u, c)
		if err != nil {
			return "", err
		}
		if role == "" {
			return "", fail(403, "仅负责人或本组织副管理员可管理平台押金")
		}
		return clubDepositAccount(clubID), nil
	}
	if c.Status != "active" {
		return "", fail(409, "俱乐部尚未开放")
	}
	if e = guardClubDeposit(ctx, q, clubID); e != nil {
		return "", e
	}
	return memberDepositAccount(clubID, u.ID), nil
}

// guardClubDepositRefund 在组织锁内同时检查认证成员与历史相关履约和售后，杜绝先退款再并发认证。
func guardClubDepositRefund(ctx context.Context, q querier, clubID string) error {
	if e := lockClubFinance(ctx, q, clubID); e != nil {
		return e
	}
	var count int
	if e := q.QueryRow(ctx, "SELECT count(*) FROM members WHERE data->>'club_id'=$1 AND data->>'status'='active'", clubID).Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return fail(409, "俱乐部仍有认证成员，请先解除全部认证后再申请退还平台押金")
	}
	orders, e := list[Order](ctx, q, "orders", "WHERE data->>'club_id'=$1 AND id NOT LIKE 'demo_%'", clubID)
	if e != nil {
		return e
	}
	for _, o := range orders {
		if orderBlocksDeposit(o) {
			return fail(409, "俱乐部存在履约或售后中的订单，暂不能退还平台押金")
		}
	}
	return nil
}

// orderBlocksDeposit 统一判断待付款、履约与七天售后是否仍占用押金保障资格。
func orderBlocksDeposit(o Order) bool {
	if o.ActiveAdjustmentID != "" {
		return true
	}
	if !contains([]string{"cancelled", "refunded", "completed"}, o.Status) || contains([]string{"requested", "escalated", "processing"}, o.RefundStatus) || o.RefundStatus == "rejected" && !o.RefundFinal {
		return true
	}
	if o.Status == "completed" {
		t, e := time.Parse(time.RFC3339Nano, o.CompletedAt)
		return e != nil || time.Now().Before(t.Add(walletHoldDuration))
	}
	return false
}

// guardMemberDepositRefund 只有退会或解除认证且无未决交易后可退会员押金，组织无扣罚或提现接口。
func guardMemberDepositRefund(ctx context.Context, q querier, club, user string) error {
	if e := lockClubFinance(ctx, q, club); e != nil {
		return e
	}
	m, e := get[ClubMember](ctx, q, "members", club+":"+user, false)
	if e != nil && !missing(e) {
		return e
	}
	if e == nil && contains([]string{"active", "pending"}, m.Status) {
		return fail(409, "请先退会或解除认证，再申请退还俱乐部押金")
	}
	return guardDepositOrders(ctx, q, user)
}

// clubDeposits 返回实缴专户数据，组织规则金额不会伪装成已缴押金。
func (a *App) clubDeposits(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	c, e := get[Club](r.Context(), a.DB, "clubs", r.PathValue("id"), false)
	if e != nil {
		return nil, e
	}
	s, e := financialSettings(r.Context(), a.DB)
	if e != nil {
		return nil, e
	}
	p, e := accountBalance(r.Context(), a.DB, clubDepositAccount(c.ID))
	if e != nil {
		return nil, e
	}
	m, e := accountBalance(r.Context(), a.DB, memberDepositAccount(c.ID, u.ID))
	required := clubRequiredDeposit(c, s)
	role, permissionErr := clubManagementRole(r.Context(), a.DB, u, c)
	if permissionErr != nil {
		return nil, permissionErr
	}
	return map[string]any{"platform": map[string]any{"amount_cents": p.DepositCents, "refunding_cents": p.DepositRefundingCents, "required_cents": required, "shortfall_cents": depositShortfall(p.DepositCents, required), "qualified": qualifiedPlatformDeposit(p, required) > 0, "can_manage": role != ""}, "membership": map[string]any{"amount_cents": m.DepositCents, "refunding_cents": m.DepositRefundingCents, "required_cents": c.DepositCents}}, e
}

// clubWalletSummary 仅负责人或平台管理可读取组织佣金账户，会员押金不会进入其中。
func (a *App) clubWalletSummary(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	c, e := a.clubManager(r, u, r.PathValue("id"))
	if e != nil {
		return nil, e
	}
	if e = a.releaseClubIncome(r.Context(), c.ID); e != nil {
		return nil, e
	}
	wallet, e := accountBalance(r.Context(), a.DB, clubWalletID(c.ID))
	if e != nil {
		return nil, e
	}
	return a.walletSummaryValue(r.Context(), wallet, c.ID)
}

// clubWalletWithdrawals 验证当前负责人后复用提现状态机，收款身份在申请时绑定且出款前复核。
func (a *App) clubWalletWithdrawals(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	c, e := a.clubManager(r, u, r.PathValue("id"))
	if e != nil {
		return nil, e
	}
	if r.Method == "POST" {
		role, permissionErr := clubManagementRole(r.Context(), a.DB, u, c)
		if permissionErr != nil {
			return nil, permissionErr
		}
		if role == "" {
			return nil, fail(403, "仅负责人或本组织副管理员可申请俱乐部提现")
		}
		owner, permissionErr := get[User](r.Context(), a.DB, "users", c.OwnerID, false)
		if permissionErr != nil || owner.Status != "active" {
			return nil, fail(409, "负责人账号不可用")
		}
		return a.createWithdrawalFor(w, r, owner, clubWalletID(c.ID), c.ID, u.ID)
	}
	limit, offset := pageLimit(r)
	items, e := list[WalletWithdrawal](r.Context(), a.DB, "wallet_withdrawals", "WHERE data->>'club_id'=$1 ORDER BY (data->>'created_at')::timestamptz DESC,id DESC LIMIT $2 OFFSET $3", c.ID, limit, offset)
	out := []any{}
	for _, v := range items {
		out = append(out, a.withdrawalResponse(v, v.UserID == u.ID))
	}
	return out, e
}

// depositRefundAccount 退款不受组织补缴或暂停招募限制，只验证原付款人对押金专户的归属。
func (a *App) depositRefundAccount(ctx context.Context, q querier, u User, purpose, club string) (string, error) {
	if purpose == "deposit" {
		if club != "" {
			return "", fail(400, "个人押金无需俱乐部编号")
		}
		return u.ID, nil
	}
	if e := lockClubFinance(ctx, q, club); e != nil {
		return "", e
	}
	c, e := get[Club](ctx, q, "clubs", club, false)
	if e != nil {
		return "", e
	}
	if purpose == "club_platform" {
		role, err := clubManagementRole(ctx, q, u, c)
		if err != nil {
			return "", err
		}
		if role == "" {
			return "", fail(403, "仅负责人或本组织副管理员可退还平台押金")
		}
		return clubDepositAccount(club), nil
	}
	return memberDepositAccount(club, u.ID), nil
}

// clubWalletEntries 仅向负责人及平台管理展示独立组织佣金流水。
func (a *App) clubWalletEntries(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.actor(r)
	if e != nil {
		return nil, e
	}
	c, e := a.clubManager(r, u, r.PathValue("id"))
	if e != nil {
		return nil, e
	}
	limit, offset := pageLimit(r)
	return list[WalletEntry](r.Context(), a.DB, "wallet_entries", "WHERE data->>'user_id'=$1 ORDER BY data->>'created_at' DESC,id DESC LIMIT $2 OFFSET $3", clubWalletID(c.ID), limit, offset)
}

// financialRules 提供公开只读费率及创建俱乐部门槛，不含任何商户配置。
func (a *App) financialRules(w http.ResponseWriter, r *http.Request) (any, error) {
	return financialSettings(r.Context(), a.DB)
}

// memberFinancialView 从实缴专户装饰认证成员视图，规则确认永远不等于已付款。
func (a *App) memberFinancialView(ctx context.Context, m ClubMember, u User, private bool) map[string]any {
	out := communityMemberView(m, u, private)
	if wallet, e := accountBalance(ctx, a.DB, memberDepositAccount(m.ClubID, m.UserID)); e == nil {
		out["club_deposit_cents"] = wallet.DepositCents
		if private && wallet.DepositCents > 0 {
			out["deposit_status"] = "paid"
		}
	}
	if private {
		out["applied_commission_rate_bps"] = m.AppliedCommissionRateBPS
		out["applied_join_note"] = m.AppliedJoinNote
	}
	return out
}
