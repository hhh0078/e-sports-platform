// Package app 管理抽成与俱乐部平台押金配置，并在每笔订单中冻结整数分分账快照。
package app

import (
	"context"
	"net/http"
)

// FinancialSettings 使用万分比表达两档平台费率，零押金配置表示不收取而非已经缴纳。
type FinancialSettings struct {
	FreePlatformRateBPS      int64 `json:"free_platform_rate_bps"`
	ClubPlatformRateBPS      int64 `json:"club_platform_rate_bps"`
	ClubCreationDepositCents int64 `json:"club_creation_deposit_cents"`
}

// OrderSettlement 保存订单创建时不可追改的分账比例与金额。
type OrderSettlement struct {
	Version             int    `json:"version"`
	FreePlatformRateBPS int64  `json:"free_platform_rate_bps"`
	ClubPlatformRateBPS int64  `json:"club_platform_rate_bps"`
	PlatformRateBPS     int64  `json:"platform_rate_bps"`
	ClubRateBPS         int64  `json:"club_rate_bps"`
	PlatformFeeCents    int64  `json:"platform_fee_cents"`
	ClubFeeCents        int64  `json:"club_fee_cents"`
	ProviderIncomeCents int64  `json:"provider_income_cents"`
	ClubID              string `json:"club_id"`
}

// financialSettings 读取财务配置，缺省不收费；历史订单不会被新配置改写。
func financialSettings(ctx context.Context, q querier) (FinancialSettings, error) {
	v, e := get[FinancialSettings](ctx, q, "settings", "financial", false)
	if missing(e) {
		return FinancialSettings{}, nil
	}
	return v, e
}

// financialSettingsAPI 只允许平台管理员读取或修改费率及创建俱乐部押金。
func (a *App) financialSettingsAPI(w http.ResponseWriter, r *http.Request) (any, error) {
	u, e := a.admin(r)
	if e != nil {
		return nil, e
	}
	if r.Method == "GET" {
		return financialSettings(r.Context(), a.DB)
	}
	var v FinancialSettings
	if e = decode(r, &v); e != nil {
		return nil, e
	}
	if v.FreePlatformRateBPS < 0 || v.FreePlatformRateBPS > 10000 || v.ClubPlatformRateBPS < 0 || v.ClubPlatformRateBPS > 10000 || v.ClubCreationDepositCents < 0 || v.ClubCreationDepositCents > 10000000 {
		return nil, fail(400, "抽成须在0至100%，俱乐部平台押金须在0至100000元")
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(r.Context())
	if e = lockFinancialSettings(r.Context(), tx); e != nil {
		return nil, e
	}
	var maxClubRate int64
	if e = tx.QueryRow(r.Context(), `SELECT COALESCE(max((data->>'commission_rate_bps')::bigint),0) FROM clubs WHERE id NOT LIKE 'demo_%'`).Scan(&maxClubRate); e != nil {
		return nil, e
	}
	if maxClubRate+v.ClubPlatformRateBPS > 10000 {
		return nil, fail(400, "现有俱乐部抽成与新平台抽成合计超过100%，请先调整俱乐部比例")
	}
	if e = put(r.Context(), tx, "settings", "financial", v); e == nil {
		e = audit(r.Context(), tx, u.ID, "financial.settings", "financial", "调整新订单抽成与俱乐部平台押金")
	}
	if e != nil {
		return nil, e
	}
	return v, tx.Commit(r.Context())
}

// freezeOrderSettlement 创建时冻结两档平台费率，领取公开需求仅选旧档位并冻结当时组织比例，余分归打手。
func freezeOrderSettlement(ctx context.Context, q querier, o *Order) error {
	original := o.Settlement
	freeRate, clubRate := original.FreePlatformRateBPS, original.ClubPlatformRateBPS
	if original.Version == 1 {
		freeRate = original.PlatformRateBPS
		clubRate = original.PlatformRateBPS
	} else if original.Version == 0 {
		if o.PaymentStatus == "paid" {
			freeRate = 0
			clubRate = 0
		} else {
			settings, e := financialSettings(ctx, q)
			if e != nil {
				return e
			}
			freeRate = settings.FreePlatformRateBPS
			clubRate = settings.ClubPlatformRateBPS
		}
	}
	if freeRate < 0 || freeRate > 10000 || clubRate < 0 || clubRate > 10000 {
		return fail(409, "订单平台费率快照无效，请联系平台核账")
	}
	s := OrderSettlement{Version: 2, FreePlatformRateBPS: freeRate, ClubPlatformRateBPS: clubRate, PlatformRateBPS: freeRate, ClubID: o.ClubID}
	if o.ClubID != "" {
		c, e := get[Club](ctx, q, "clubs", o.ClubID, false)
		if e != nil {
			return e
		}
		s.PlatformRateBPS = clubRate
		s.ClubRateBPS = c.CommissionRateBPS
	}
	if s.PlatformRateBPS < 0 || s.ClubRateBPS < 0 || s.PlatformRateBPS+s.ClubRateBPS > 10000 {
		return fail(409, "平台与俱乐部抽成比例合计超过100%，暂不可接单")
	}
	s.PlatformFeeCents = o.AmountCents * s.PlatformRateBPS / 10000
	s.ClubFeeCents = o.AmountCents * s.ClubRateBPS / 10000
	s.ProviderIncomeCents = o.AmountCents - s.PlatformFeeCents - s.ClubFeeCents
	o.Settlement = s
	return nil
}

// settlementForOrder 为历史无快照订单保留零抽成，禁止将新费率追溯到旧订单。
func settlementForOrder(o Order) OrderSettlement {
	if o.Settlement.Version > 0 {
		return o.Settlement
	}
	return OrderSettlement{ProviderIncomeCents: o.AmountCents, ClubID: o.ClubID}
}

// clubWalletID 返回独立于负责人个人余额的组织财务账号。
func clubWalletID(id string) string { return "club:" + id }

// lockFinancialSettings 串行校验平台与组织费率修改，避免两边同时保存导致合计超限。
func lockFinancialSettings(ctx context.Context, q querier) error {
	_, e := q.Exec(ctx, "SELECT pg_advisory_xact_lock(8173019)")
	return e
}

// clubRequiredDeposit 对未公开创建草稿保留门槛快照，已运行组织按当前平台补缴规则判断。
func clubRequiredDeposit(c Club, s FinancialSettings) int64 {
	if contains([]string{"pending_deposit", "pending", "rejected"}, c.Status) && c.CreationDepositVersion > 0 {
		return c.CreationDepositRequiredCents
	}
	return s.ClubCreationDepositCents
}
