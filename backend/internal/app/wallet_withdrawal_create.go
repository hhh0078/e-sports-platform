// Package app 以逐笔资金来源核验、账户冻结和上海自然日限制创建本人或组织提现申请。
package app

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// createWalletWithdrawal 在同一账号锁下检查服务收入并冻结；充值款、押金和待结算均不可提现。
func (a *App) createWalletWithdrawal(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	return a.createWithdrawalFor(w, r, u, u.ID, "")
}

// createWithdrawalFor 为本人或负责人的组织独立账户冻结可核验来源，并原子限制每日有效申请次数。
func (a *App) createWithdrawalFor(w http.ResponseWriter, r *http.Request, u User, accountID, clubID string, requestedBy ...string) (any, error) {
	var err error
	requesterID := u.ID
	if len(requestedBy) > 0 {
		requesterID = requestedBy[0]
	}
	var in struct {
		AmountCents int64  `json:"amount_cents"`
		RequestID   string `json:"request_id"`
		UserName    string `json:"user_name"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if err = walletRequestID(in.RequestID); err != nil {
		return nil, err
	}
	in.UserName = strings.TrimSpace(in.UserName)
	if in.AmountCents < 1 || in.AmountCents > 20000000 || !length(in.UserName, 0, 50) {
		return nil, fail(400, "提现金额必须为正数且不超过200000元，实名最多50字")
	}
	if in.AmountCents >= 200000 && in.UserName == "" {
		return nil, fail(400, "微信要求单笔2000元及以上提现填写真实姓名")
	}
	if !a.Config.DevMode && in.AmountCents < 10 {
		return nil, fail(400, "微信实际转账最低0.1元，平台不另设最低金额")
	}
	if !a.Config.DevMode && clubID != "" && (in.AmountCents < 30 || in.UserName == "") {
		return nil, fail(400, "俱乐部提现须核验负责人微信实名，请填写真实姓名且金额至少0.3元")
	}
	if !a.Config.DevMode && in.AmountCents < 30 && in.UserName != "" {
		return nil, fail(400, "微信不支持低于0.3元的姓名校验，请清除姓名或提高金额")
	}
	var settings WalletSettings
	var c wxPayConfig
	if !a.Config.DevMode {
		settings, c, err = a.readyWalletTransfer(r.Context())
		if err != nil {
			return nil, err
		}
	}
	if clubID != "" {
		err = a.releaseClubIncome(r.Context(), clubID)
	} else {
		err = a.releaseWalletIncome(r.Context(), u.ID)
	}
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockClubFinance(ctx, tx, clubID); err != nil {
		return nil, err
	}
	if err = lockWalletUsers(ctx, tx, u.ID, accountID, requesterID); err != nil {
		return nil, err
	}
	if clubID != "" {
		club, e := get[Club](ctx, tx, "clubs", clubID, false)
		if e != nil {
			return nil, e
		}
		if club.OwnerID != u.ID || accountID != clubWalletID(club.ID) {
			return nil, fail(409, "负责人或组织资金账号发生变化")
		}
		requester, e := get[User](ctx, tx, "users", requesterID, false)
		if e != nil {
			return nil, e
		}
		role, e := clubManagementRole(ctx, tx, requester, club)
		if e != nil {
			return nil, e
		}
		if role == "" {
			return nil, fail(403, "本组织资金申请权限已失效")
		}
		owner, e := get[User](ctx, tx, "users", u.ID, false)
		if e != nil || owner.Status != "active" {
			return nil, fail(409, "负责人账号不可用")
		}
	}
	if err = claimWalletRequest(ctx, tx, requesterID, in.RequestID, "withdrawal", map[string]any{"name_hash": walletKey(in.UserName), "amount_cents": in.AmountCents, "account_id": accountID}); err != nil {
		return nil, err
	}
	id := walletKey("withdrawal", requesterID, in.RequestID)
	v, err := get[WalletWithdrawal](ctx, tx, "wallet_withdrawals", id, true)
	if err == nil {
		if withdrawalAccount(v) != accountID || v.AmountCents != in.AmountCents || v.NameHash != walletKey(in.UserName) {
			return nil, fail(409, "同一request_id不能修改提现金额或实名")
		}
		return a.withdrawalResponse(v, requesterID == v.UserID), nil
	}
	if !missing(err) {
		return nil, err
	}
	if err = guardDailyWithdrawal(ctx, tx, accountID, time.Now()); err != nil {
		return nil, err
	}
	v = WalletWithdrawal{AccountID: accountID, AccountType: "user", ClubID: clubID, ID: id, UserID: u.ID, RequestID: in.RequestID, AmountCents: in.AmountCents, Status: "pending_review", OutBillNo: "WD" + strings.ToUpper(id[:30]), NameHash: walletKey(in.UserName), TransferConfig: settings, CreatedAt: now(), UpdatedAt: now()}
	v.RequestedBy = requesterID
	if clubID != "" {
		v.AccountType = "club"
	}
	if !a.Config.DevMode {
		identity, e := get[map[string]string](ctx, tx, "identities", u.ID, false)
		if e != nil || identity["openid"] == "" {
			return nil, fail(503, "请先通过微信登录后申请提现")
		}
		v.PayOpenID = identity["openid"]
		v.MerchantID = c.MerchantID
		v.AppID = c.AppID
		if in.UserName != "" {
			encrypted, e := rsa.EncryptOAEP(sha1.New(), rand.Reader, c.PublicKey, []byte(in.UserName), nil)
			if e != nil {
				return nil, fail(503, "实名加密失败，请检查微信支付公钥")
			}
			v.EncryptedName = base64.StdEncoding.EncodeToString(encrypted)
		}
	}
	wallet, err := lockedWallet(ctx, tx, accountID)
	if err != nil {
		return nil, err
	}
	if err = moveFunding(ctx, tx, "withdrawal-freeze:"+id, accountID, "available", in.AmountCents, []fundDestination{{AccountID: "withdrawal:" + id, Bucket: "withdrawal", AmountCents: in.AmountCents}}); err != nil {
		return nil, err
	}
	if sources, issues, e := auditWithdrawalSources(ctx, tx, v, a.Config.DevMode); e != nil {
		return nil, e
	} else if len(issues) > 0 {
		return nil, fail(409, strings.Join(issues, "；"))
	} else if e = recordFundingAudit(ctx, tx, v, "request", sources, issues); e != nil {
		return nil, e
	}
	if err = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "withdrawal_freeze", ReferenceID: id, AmountCents: in.AmountCents, AvailableDelta: -in.AmountCents, WithdrawalFrozenDelta: in.AmountCents, Note: "提现申请，冻结服务收入等待审核"}); err != nil {
		return nil, err
	}
	if err = put(ctx, tx, "wallet_withdrawals", id, v); err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, requesterID, "wallet.withdrawal.request", v.ID, "申请人和原负责人收款身份独立绑定"); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return a.withdrawalResponse(v, requesterID == v.UserID), nil
}
