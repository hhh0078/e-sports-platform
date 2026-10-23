// Package app 以用户事务锁、整数分校验和不可变业务流水保护钱包资金一致性。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"sort"
	"time"
)

const walletHoldDuration = 7 * 24 * time.Hour

var walletRequestPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{7,79}$`)

// walletKey 根据内部业务用途生成稳定主键，不使用客户端提供的用户或金额作为权限依据。
func walletKey(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// lockWalletUsers 按固定用户顺序取得事务锁；新订单、支付、接单和退押金共同遵守此入口。
func lockWalletUsers(ctx context.Context, q querier, userIDs ...string) error {
	ids := append([]string{}, userIDs...)
	sort.Strings(ids)
	previous := ""
	for _, id := range ids {
		if id == "" || id == previous {
			continue
		}
		if _, err := q.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "jijie-wallet:"+id); err != nil {
			return err
		}
		previous = id
	}
	return nil
}

// lockWalletOrder 先统一锁交易相关用户，再锁订单；接单归属变化时拒绝旧快照，避免交叉锁与错误结算。
func lockWalletOrder(ctx context.Context, q querier, id string, extraUser string) (Order, error) {
	before, err := get[Order](ctx, q, "orders", id, false)
	if err != nil {
		return before, err
	}
	if err = lockClubFinance(ctx, q, before.ClubID); err != nil {
		return before, err
	}
	if err = lockWalletUsers(ctx, q, before.BuyerID, before.ProviderID, extraUser, clubWalletID(before.ClubID)); err != nil {
		return before, err
	}
	current, err := get[Order](ctx, q, "orders", id, true)
	if err == nil && (current.BuyerID != before.BuyerID || current.ProviderID != before.ProviderID) {
		return current, fail(409, "订单归属已更新，请刷新后重试")
	}
	if err == nil {
		err = expireOrderAdjustment(ctx, q, &current)
	}
	return current, err
}

// lockedWallet 创建零余额账户并取得行锁，调用者必须先持有对应用户事务锁。
func lockedWallet(ctx context.Context, q querier, userID string) (Wallet, error) {
	initial := Wallet{UserID: userID, UpdatedAt: now()}
	raw, err := json.Marshal(initial)
	if err != nil {
		return initial, err
	}
	if _, err = q.Exec(ctx, "INSERT INTO wallets(id,data) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", userID, raw); err != nil {
		return initial, err
	}
	return get[Wallet](ctx, q, "wallets", userID, true)
}

// checkedWalletAmount 拒绝负余额与整数溢出，金额永不经过浮点转换。
func checkedWalletAmount(current, delta int64) (int64, error) {
	if delta > 0 && current > math.MaxInt64-delta || delta < 0 && current < math.MinInt64-delta {
		return 0, fail(409, "钱包金额超出可用范围")
	}
	result := current + delta
	if result < 0 {
		return 0, fail(409, "余额不足或资金已被其他操作冻结")
	}
	return result, nil
}

// appendWalletEntry 只在业务流水首次插入时改变余额，同一业务号的金额冲突会中止整个事务。
func appendWalletEntry(ctx context.Context, q querier, wallet *Wallet, entry WalletEntry) error {
	entry.UserID = wallet.UserID
	entry.ID = walletKey(entry.UserID, entry.Kind, entry.ReferenceID)
	entry.CreatedAt = now()
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	result, err := q.Exec(ctx, "INSERT INTO wallet_entries(id,data) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", entry.ID, raw)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		existing, readErr := get[WalletEntry](ctx, q, "wallet_entries", entry.ID, false)
		if readErr != nil {
			return readErr
		}
		existing.CreatedAt = entry.CreatedAt
		if existing != entry {
			return fail(409, "同一财务业务号的金额或用途不一致")
		}
		return nil
	}
	pairs := []struct {
		value *int64
		delta int64
	}{{&wallet.RechargeCents, entry.RechargeDelta}, {&wallet.AvailableCents, entry.AvailableDelta}, {&wallet.PendingCents, entry.PendingDelta}, {&wallet.WithdrawalFrozenCents, entry.WithdrawalFrozenDelta}, {&wallet.DepositCents, entry.DepositDelta}, {&wallet.DepositRefundingCents, entry.DepositRefundingDelta}}
	for _, pair := range pairs {
		value, addErr := checkedWalletAmount(*pair.value, pair.delta)
		if addErr != nil {
			return addErr
		}
		*pair.value = value
	}
	wallet.UpdatedAt = entry.CreatedAt
	return put(ctx, q, "wallets", wallet.UserID, *wallet)
}

// guardDepositRefunding 在同一事务锁下阻止退押金期间创建、支付或领取新的订单。
func guardDepositRefunding(ctx context.Context, q querier, userIDs ...string) error {
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		wallet, err := get[Wallet](ctx, q, "wallets", userID, false)
		if missing(err) {
			continue
		}
		if err != nil {
			return err
		}
		if wallet.DepositRefundingCents > 0 {
			return fail(409, "相关账号正在退还平台押金，请等待退款完成后再创建、支付或领取新订单")
		}
	}
	return nil
}

// walletSummary 读取前释放已满售后期且无争议的收入，返回用途清晰的资金分区。
func (a *App) walletSummary(w http.ResponseWriter, r *http.Request) (any, error) {
	user, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if err = a.releaseWalletIncome(r.Context(), user.ID); err != nil {
		return nil, err
	}
	wallet, err := get[Wallet](r.Context(), a.DB, "wallets", user.ID, false)
	if missing(err) {
		wallet = Wallet{UserID: user.ID}
	} else if err != nil {
		return nil, err
	}
	return a.walletSummaryValue(r.Context(), wallet, "")
}

// walletEntries 返回本人不可变账单，分页始终在账号过滤之后执行。
func (a *App) walletEntries(w http.ResponseWriter, r *http.Request) (any, error) {
	user, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	return list[WalletEntry](r.Context(), a.DB, "wallet_entries", "WHERE data->>'user_id'=$1 ORDER BY (data->>'created_at')::timestamptz DESC,id DESC LIMIT $2 OFFSET $3", user.ID, limit, offset)
}

// walletRequestID 校验客户端幂等号，仅用于区分同一账号的请求，不构成身份凭证。
func walletRequestID(value string) error {
	if !walletRequestPattern.MatchString(value) {
		return fail(400, "请提供8至80位字母、数字、横线或下划线组成的request_id")
	}
	return nil
}

// claimWalletRequest 把同一账号幂等号绑定至唯一资金用途与规范载荷，跨订单、跨用途复用均返回冲突。
func claimWalletRequest(ctx context.Context, q querier, userID, requestID, kind string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	expected := map[string]string{"user_id": userID, "request_id": requestID, "kind": kind, "payload": string(raw)}
	id := walletKey("request", userID, requestID)
	existing, err := get[map[string]string](ctx, q, "wallet_requests", id, false)
	if err == nil {
		if existing["kind"] != kind || existing["payload"] != string(raw) {
			return fail(409, "同一request_id不能用于不同资金用途、订单或金额")
		}
		return nil
	}
	if !missing(err) {
		return err
	}
	return put(ctx, q, "wallet_requests", id, expected)
}

// walletSummaryValue 将未履约净收入与已完成待售后净收入分开展示，不重复计算总余额。
func (a *App) walletSummaryValue(ctx context.Context, wallet Wallet, clubID string) (any, error) {
	spendable, e := checkedWalletAmount(wallet.RechargeCents, wallet.AvailableCents)
	if e != nil {
		return nil, e
	}
	where := "WHERE data->>'provider_id'=$1 AND data->>'status'='holding'"
	arg := wallet.UserID
	if clubID != "" {
		where = "WHERE data->>'status'='holding' AND EXISTS(SELECT 1 FROM orders o WHERE o.id=wallet_escrows.id AND o.data->>'club_id'=$1)"
		arg = clubID
	}
	escrows, e := list[WalletEscrow](ctx, a.DB, "wallet_escrows", where, arg)
	if e != nil {
		return nil, e
	}
	var ongoing, settling int64
	for _, esc := range escrows {
		o, e := get[Order](ctx, a.DB, "orders", esc.OrderID, false)
		if e != nil {
			return nil, e
		}
		s := settlementForOrder(o)
		amount := s.ProviderIncomeCents
		if clubID != "" {
			amount = s.ClubFeeCents
		}
		if o.Status == "completed" {
			settling += amount
		} else {
			ongoing += amount
		}
	}
	minimum := int64(1)
	nameRequired := false
	if !a.Config.DevMode {
		minimum = 10
		if clubID != "" {
			minimum = 30
			nameRequired = true
		}
	}
	daily := guardDailyWithdrawal(ctx, a.DB, wallet.UserID, time.Now()) == nil
	settings, e := financialSettings(ctx, a.DB)
	if e != nil {
		return nil, e
	}
	depositRequired := settings.PersonalPlatformDepositCents
	if clubID != "" {
		depositRequired = 0
	}
	return map[string]any{"withdrawal_min_cents": minimum, "withdrawal_name_required": nameRequired, "daily_withdrawal_available": daily, "available_cents": wallet.AvailableCents, "recharge_cents": wallet.RechargeCents, "spendable_cents": spendable, "pending_cents": wallet.PendingCents, "ongoing_cents": ongoing, "settling_cents": settling, "withdrawal_frozen_cents": wallet.WithdrawalFrozenCents, "deposit_cents": wallet.DepositCents, "deposit_refunding_cents": wallet.DepositRefundingCents, "deposit_required_cents": depositRequired, "deposit_shortfall_cents": depositShortfall(wallet.DepositCents, depositRequired), "deposit_qualified": qualifiedPlatformDeposit(wallet, depositRequired) > 0, "currency": "CNY", "settlement_hold_days": 7, "dev_mode": a.Config.DevMode, "club_id": clubID}, nil
}
