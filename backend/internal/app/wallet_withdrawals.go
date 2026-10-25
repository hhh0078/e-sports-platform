// Package app 先冻结可提现服务收入，再通过微信商家转账的核验终态扣除或解冻，未知外部结果持续冻结。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// wxTransfer 同时接收微信查询与通知中的转账资料，付款结果必须与原申请逐项匹配。
type wxTransfer struct {
	OutBillNo      string `json:"out_bill_no"`
	TransferBillNo string `json:"transfer_bill_no"`
	State          string `json:"state"`
	MerchantID     string `json:"mch_id"`
	AppID          string `json:"appid"`
	OpenID         string `json:"openid"`
	AmountCents    int64  `json:"transfer_amount"`
	PackageInfo    string `json:"package_info"`
	FailReason     string `json:"fail_reason"`
}

// withdrawalResponse 仅向本人返回待确认收款参数，管理列表及其他状态不暴露微信身份或实名密文。
func (a *App) withdrawalResponse(v WalletWithdrawal, own bool) any {
	if v.AccountType == "" {
		v.AccountType = "user"
	}
	pack := v.PackageInfo
	safe := publicValue(v).(WalletWithdrawal)
	raw, _ := json.Marshal(safe)
	out := map[string]any{}
	json.Unmarshal(raw, &out)
	if own && v.Status == "wait_user_confirm" && pack != "" {
		if c, err := a.loadWXPay(); err == nil && (v.MerchantID == "" || v.MerchantID == c.MerchantID && v.AppID == c.AppID) {
			out["confirmation"] = map[string]string{"mch_id": c.MerchantID, "app_id": c.AppID, "package_info": pack}
		}
	}
	return out
}

// walletWithdrawals 返回本人提现申请；只有本人待确认收款记录带微信拉起参数。
func (a *App) walletWithdrawals(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	items, err := list[WalletWithdrawal](r.Context(), a.DB, "wallet_withdrawals", "WHERE data->>'user_id'=$1 ORDER BY (data->>'created_at')::timestamptz DESC,id DESC LIMIT $2 OFFSET $3", u.ID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := []any{}
	for _, v := range items {
		out = append(out, a.withdrawalResponse(v, true))
	}
	return out, nil
}

// adminWalletWithdrawals 对管理员开放资金审核列表，仍剥离所有微信身份和实名资料。
func (a *App) adminWalletWithdrawals(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	where := ""
	args := []any{limit, offset}
	if status := r.URL.Query().Get("status"); status != "" && status != "all" {
		if !contains([]string{"pending_review", "processing", "wait_user_confirm", "success", "failed", "rejected"}, status) {
			return nil, fail(400, "提现状态无效")
		}
		where = "WHERE data->>'status'=$3"
		args = append(args, status)
	}
	return list[WalletWithdrawal](r.Context(), a.DB, "wallet_withdrawals", where+" ORDER BY (data->>'created_at')::timestamptz DESC,id DESC LIMIT $1 OFFSET $2", args...)
}

// finishWalletWithdrawal 只用已核验终态处理冻结款，重复成功或失败通知不会再次扣除或解冻。
func (a *App) finishWalletWithdrawal(ctx context.Context, snapshot WalletWithdrawal, result wxTransfer, dev bool) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockClubFinance(ctx, tx, snapshot.ClubID); err != nil {
		return err
	}
	if err = lockWalletUsers(ctx, tx, snapshot.UserID, withdrawalAccount(snapshot)); err != nil {
		return err
	}
	v, err := get[WalletWithdrawal](ctx, tx, "wallet_withdrawals", snapshot.ID, true)
	if err != nil {
		return err
	}
	if !dev && ((v.MerchantID != "" && v.MerchantID != result.MerchantID) || (v.AppID != "" && result.AppID != "" && v.AppID != result.AppID) || result.OutBillNo != v.OutBillNo || result.OpenID != v.PayOpenID || result.AmountCents != v.AmountCents || result.TransferBillNo == "") {
		return fail(400, "提现金额、收款人或微信流水不匹配")
	}
	if v.TransferBillNo != "" && v.TransferBillNo != result.TransferBillNo {
		return fail(409, "微信转账流水冲突")
	}
	terminal := contains([]string{"success", "failed", "rejected"}, v.Status)
	desired := "processing"
	switch result.State {
	case "SUCCESS":
		desired = "success"
	case "FAIL", "CANCELLED":
		desired = "failed"
	case "WAIT_USER_CONFIRM":
		desired = "wait_user_confirm"
	case "ACCEPTED", "PROCESSING", "TRANSFERING", "CANCELING":
	default:
		return fail(400, "微信转账状态无效")
	}
	if terminal {
		if v.Status == desired {
			return nil
		}
		return fail(409, "提现终态冲突，请人工核查")
	}
	if v.Status == "pending_review" {
		return fail(409, "尚未审核发起的提现不能接收转账结果")
	}
	if desired == "success" || desired == "failed" {
		wallet, e := lockedWallet(ctx, tx, withdrawalAccount(v))
		if e != nil {
			return e
		}
		entry := WalletEntry{Kind: "withdrawal_finish", ReferenceID: v.ID, AmountCents: -v.AmountCents, WithdrawalFrozenDelta: -v.AmountCents, Note: "微信已确认提现成功"}
		if desired == "failed" {
			entry.AmountCents = v.AmountCents
			entry.AvailableDelta = v.AmountCents
			entry.Note = "微信确认转账失败或取消，解冻服务收入"
		}
		if err = finishWithdrawalSources(ctx, tx, v, desired == "success"); err != nil {
			return err
		}
		if err = appendWalletEntry(ctx, tx, &wallet, entry); err != nil {
			return err
		}
	}
	v.Status = desired
	v.TransferBillNo = result.TransferBillNo
	v.PackageInfo = result.PackageInfo
	v.FailureReason = result.FailReason
	v.UpdatedAt = now()
	if desired != "wait_user_confirm" {
		v.PackageInfo = ""
	}
	if err = put(ctx, tx, "wallet_withdrawals", v.ID, v); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// rejectWalletWithdrawal 只允许拒绝尚未出网的待审核申请，未知微信转账永不由本地按钮直接解冻。
func (a *App) rejectWalletWithdrawal(w http.ResponseWriter, r *http.Request) (any, error) {
	admin, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !length(in.Reason, 2, 500) {
		return nil, fail(400, "请填写2至500字拒绝原因")
	}
	ctx := r.Context()
	before, err := get[WalletWithdrawal](ctx, a.DB, "wallet_withdrawals", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockClubFinance(ctx, tx, before.ClubID); err != nil {
		return nil, err
	}
	if err = lockWalletUsers(ctx, tx, before.UserID, withdrawalAccount(before)); err != nil {
		return nil, err
	}
	admin, err = lockManagementWriteActor(ctx, tx, admin.ID)
	if err != nil {
		return nil, err
	}
	if admin.Role != "admin" {
		return nil, fail(403, "提现审核权限已失效")
	}
	v, err := get[WalletWithdrawal](ctx, tx, "wallet_withdrawals", before.ID, true)
	if err != nil {
		return nil, err
	}
	if v.Status == "rejected" {
		return v, nil
	}
	if v.Status != "pending_review" {
		return nil, fail(409, "仅尚未发起微信转账的申请可以审核拒绝")
	}
	wallet, err := lockedWallet(ctx, tx, withdrawalAccount(v))
	if err != nil {
		return nil, err
	}
	if err = finishWithdrawalSources(ctx, tx, v, false); err != nil {
		return nil, err
	}
	if err = appendWalletEntry(ctx, tx, &wallet, WalletEntry{Kind: "withdrawal_reject", ReferenceID: v.ID, AmountCents: v.AmountCents, AvailableDelta: v.AmountCents, WithdrawalFrozenDelta: -v.AmountCents, Note: "平台拒绝提现申请，解冻服务收入"}); err != nil {
		return nil, err
	}
	v.Status = "rejected"
	v.FailureReason = in.Reason
	v.UpdatedAt = now()
	if err = put(ctx, tx, "wallet_withdrawals", v.ID, v); err != nil {
		return nil, err
	}
	if err = audit(ctx, tx, admin.ID, "wallet.withdrawal.reject", v.ID, in.Reason); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return v, nil
}

// executeWalletWithdrawal 在审核事务提交后调用官方接口，网络超时保持冻结并依赖原商户单号查询补偿。
func (a *App) executeWalletWithdrawal(w http.ResponseWriter, r *http.Request) (any, error) {
	admin, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		DevOutcome string `json:"dev_outcome"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if a.Config.DevMode && in.DevOutcome != "success" && in.DevOutcome != "failed" {
		return nil, fail(400, "开发提现请选择success或failed")
	}
	if !a.Config.DevMode && in.DevOutcome != "" {
		return nil, fail(400, "生产环境不接受模拟提现结果")
	}
	ctx := r.Context()
	before, err := get[WalletWithdrawal](ctx, a.DB, "wallet_withdrawals", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if contains([]string{"success", "failed", "rejected"}, before.Status) {
		return before, nil
	}
	var c wxPayConfig
	if !a.Config.DevMode {
		if _, c, err = a.readyWalletTransfer(ctx); err != nil {
			return nil, err
		}
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = lockClubFinance(ctx, tx, before.ClubID); err != nil {
		return nil, err
	}
	if err = lockWalletUsers(ctx, tx, before.UserID, withdrawalAccount(before)); err != nil {
		return nil, err
	}
	admin, err = lockManagementWriteActor(ctx, tx, admin.ID)
	if err != nil {
		return nil, err
	}
	if admin.Role != "admin" {
		return nil, fail(403, "提现审核权限已失效")
	}
	v, err := get[WalletWithdrawal](ctx, tx, "wallet_withdrawals", before.ID, true)
	if err != nil {
		return nil, err
	}
	if !a.Config.DevMode && v.MerchantID != "" && (v.MerchantID != c.MerchantID || v.AppID != c.AppID) {
		return nil, fail(409, "商户配置已变更，请先核查原提现申请，不可换商户重发")
	}
	if v.ClubID != "" {
		club, e := get[Club](ctx, tx, "clubs", v.ClubID, false)
		if e != nil || club.OwnerID != v.UserID {
			return nil, fail(409, "俱乐部负责人已变更，请拒绝此申请后由新负责人申请")
		}
	}
	if sources, issues, e := auditWithdrawalSources(ctx, tx, v, a.Config.DevMode); e != nil {
		return nil, e
	} else if len(issues) > 0 {
		return nil, fail(409, strings.Join(issues, "；"))
	} else if e = recordFundingAudit(ctx, tx, v, "execute", sources, issues); e != nil {
		return nil, e
	}
	first := v.Status == "pending_review"
	if first {
		v.Status = "processing"
		v.UpdatedAt = now()
		if err = put(ctx, tx, "wallet_withdrawals", v.ID, v); err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, admin.ID, "wallet.withdrawal.execute", v.ID, "审核通过并发起商家转账"); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	if a.Config.DevMode {
		if in.DevOutcome != "success" && in.DevOutcome != "failed" {
			return nil, fail(400, "开发提现请选择success或failed")
		}
		state := "SUCCESS"
		if in.DevOutcome == "failed" {
			state = "FAIL"
		}
		err = a.finishWalletWithdrawal(ctx, v, wxTransfer{State: state, TransferBillNo: "DEV" + v.ID}, true)
	} else {
		var result wxTransfer
		if !first {
			err = a.wxRequest(ctx, c, "GET", "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/"+url.PathEscape(v.OutBillNo), nil, &result)
			if err == nil {
				err = a.applyWXTransferQuery(ctx, c, v, result)
			} else {
				var apiErr *wxAPIError
				if !errors.As(err, &apiErr) || !contains([]string{"NOT_FOUND", "RESOURCE_NOT_EXISTS"}, apiErr.Code) {
					return nil, err
				}
				first = true
			}
		}
		if first {
			payload := map[string]any{"appid": c.AppID, "out_bill_no": v.OutBillNo, "transfer_scene_id": v.TransferConfig.TransferSceneID, "openid": v.PayOpenID, "transfer_amount": v.AmountCents, "transfer_remark": v.TransferConfig.TransferRemark, "notify_url": v.TransferConfig.TransferNotifyURL, "transfer_scene_report_infos": v.TransferConfig.TransferSceneReportInfos}
			if v.EncryptedName != "" {
				payload["user_name"] = v.EncryptedName
			}
			if v.TransferConfig.UserRecvPerception != "" {
				payload["user_recv_perception"] = v.TransferConfig.UserRecvPerception
			}
			if err = a.wxRequest(ctx, c, "POST", "/v3/fund-app/mch-transfer/transfer-bills", payload, &result); err != nil {
				return nil, err
			}
			if result.OutBillNo != v.OutBillNo || result.TransferBillNo == "" {
				return nil, fail(502, "微信商家转账响应流水不匹配，资金继续冻结")
			}
			// 创建响应不含收款人和金额；必须查询完整资源后才能扣除或解冻资金。
			err = a.syncWXTransfer(ctx, c, v)
		}
	}
	if err != nil {
		return nil, err
	}
	return get[WalletWithdrawal](ctx, a.DB, "wallet_withdrawals", v.ID, false)
}

// syncWXTransfer 通过原商户转账单号查询并验证完整资金字段。
func (a *App) syncWXTransfer(ctx context.Context, c wxPayConfig, v WalletWithdrawal) error {
	var result wxTransfer
	if err := a.wxRequest(ctx, c, "GET", "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/"+url.PathEscape(v.OutBillNo), nil, &result); err != nil {
		return err
	}
	return a.applyWXTransferQuery(ctx, c, v, result)
}

// applyWXTransferQuery 对已验签的原商户单查询核对必填字段；官方允许省略openid，省略时沿用本单已绑定的收款人。
func (a *App) applyWXTransferQuery(ctx context.Context, c wxPayConfig, v WalletWithdrawal, result wxTransfer) error {
	if result.OutBillNo != v.OutBillNo || result.AppID != c.AppID || result.MerchantID != c.MerchantID || result.AmountCents != v.AmountCents || result.TransferBillNo == "" {
		return fail(400, "微信转账查询的商户、应用、金额或原单不匹配")
	}
	if result.OpenID == "" {
		result.OpenID = v.PayOpenID
	}
	return a.applyWXTransfer(ctx, c, result)
}

// applyWXTransfer 核验商户、金额、收款人以及原商户单号后更新提现状态，通知与查询共用同一入口。
func (a *App) applyWXTransfer(ctx context.Context, c wxPayConfig, result wxTransfer) error {
	if result.MerchantID != c.MerchantID || result.AppID != "" && result.AppID != c.AppID {
		return fail(400, "转账商户或小程序不匹配")
	}
	var id string
	if err := a.DB.QueryRow(ctx, "SELECT id FROM wallet_withdrawals WHERE data->>'out_bill_no'=$1", result.OutBillNo).Scan(&id); err != nil {
		return fail(404, "提现申请不存在")
	}
	v, err := get[WalletWithdrawal](ctx, a.DB, "wallet_withdrawals", id, false)
	if err != nil {
		return err
	}
	return a.finishWalletWithdrawal(ctx, v, result, false)
}

// syncWalletWithdrawal 本人按原申请查询收款结果，任何本地等待时间均不会把未知结果解冻。
func (a *App) syncWalletWithdrawal(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	v, err := get[WalletWithdrawal](r.Context(), a.DB, "wallet_withdrawals", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	own := v.UserID == u.ID
	if !own {
		if v.ClubID == "" {
			return nil, fail(403, "无权查询他人提现")
		}
		club, e := get[Club](r.Context(), a.DB, "clubs", v.ClubID, false)
		if e != nil {
			return nil, e
		}
		role, e := clubManagementRole(r.Context(), a.DB, u, club)
		if e != nil {
			return nil, e
		}
		if role != "deputy" {
			return nil, fail(403, "无权查询他人提现")
		}
	}
	if !a.Config.DevMode && contains([]string{"processing", "wait_user_confirm"}, v.Status) {
		c, e := a.loadWXPay()
		if e != nil {
			return nil, e
		}
		if err = a.syncWXTransfer(r.Context(), c, v); err != nil {
			return nil, err
		}
		v, err = get[WalletWithdrawal](r.Context(), a.DB, "wallet_withdrawals", v.ID, false)
	}
	return a.withdrawalResponse(v, own), err
}

// transferCallback 先验签解密微信商家转账通知，再用相同账户锁验证并处理官方终态。
func (a *App) transferCallback(w http.ResponseWriter, r *http.Request) {
	c, body, err := a.readWXNotify(w, r)
	var result wxTransfer
	if err == nil {
		err = decryptWXResource(c, body, "MCHTRANSFER.BILL.FINISHED", &result)
	}
	if err == nil {
		err = a.applyWXTransfer(r.Context(), c, result)
	}
	wxNotifyResult(w, err)
}

// withdrawalAccount 兼容旧个人提现并固定原申请资金账号。
func withdrawalAccount(v WalletWithdrawal) string {
	if v.AccountID != "" {
		return v.AccountID
	}
	return v.UserID
}

// guardDailyWithdrawal 按上海自然日限制有效申请，失败或审核拒绝不消耗次数，幂等重放由上层提前返回。
func guardDailyWithdrawal(ctx context.Context, q querier, account string, stamp time.Time) error {
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	local := stamp.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	var n int
	e := q.QueryRow(ctx, `SELECT count(*) FROM wallet_withdrawals WHERE COALESCE(NULLIF(data->>'account_id',''),data->>'user_id')=$1 AND data->>'status' NOT IN ('failed','rejected') AND (data->>'created_at')::timestamptz >= $2 AND (data->>'created_at')::timestamptz < $3`, account, start, start.AddDate(0, 0, 1)).Scan(&n)
	if e != nil {
		return e
	}
	if n > 0 {
		return fail(409, "每天可提现一次，今日已有处理中或成功的提现申请")
	}
	return nil
}

// finishWithdrawalSources 成功消费已冻结来源，明确失败则原样恢复同一账户的服务收入来源。
func finishWithdrawalSources(ctx context.Context, q querier, v WalletWithdrawal, success bool) error {
	targets := []fundDestination(nil)
	if !success {
		targets = []fundDestination{{AccountID: withdrawalAccount(v), Bucket: "available", AmountCents: v.AmountCents}}
	}
	return moveFunding(ctx, q, "withdrawal-finish:"+v.ID, "withdrawal:"+v.ID, "withdrawal", v.AmountCents, targets)
}

// adminWithdrawalAudit 提供个人和俱乐部统一来源核验详情，只有平台管理员可查询。
func (a *App) adminWithdrawalAudit(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, e := a.admin(r); e != nil {
		return nil, e
	}
	v, e := get[WalletWithdrawal](r.Context(), a.DB, "wallet_withdrawals", r.PathValue("id"), false)
	if e != nil {
		return nil, e
	}
	sources, issues, e := auditWithdrawalSources(r.Context(), a.DB, v, a.Config.DevMode)
	return map[string]any{"withdrawal": a.withdrawalResponse(v, false), "passed": len(issues) == 0, "issues": issues, "sources": sources, "total_cents": v.AmountCents}, e
}
