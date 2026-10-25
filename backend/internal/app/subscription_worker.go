// Package app 在业务事务外有限并发发送微信订阅消息，未知结果不盲目重发且凭据仅发送给官方固定地址。
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// StartSubscriptionWorker 启动两个有界消费者；配置关闭时仍保留明确的跳过记录。
func (a *App) StartSubscriptionWorker(ctx context.Context) {
	a.subscriptionWorkerMu.Lock()
	defer a.subscriptionWorkerMu.Unlock()
	if a.subscriptionWorkerCancel != nil {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	a.subscriptionWorkerCancel = cancel
	for i := 0; i < 2; i++ {
		a.subscriptionWorkerWG.Add(1)
		go func() {
			defer a.subscriptionWorkerWG.Done()
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				if _, err := a.processSubscriptionOne(workerCtx); err != nil && workerCtx.Err() == nil {
					log.Print("订阅消息后台处理暂缓，请检查存储状态")
				}
				select {
				case <-workerCtx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
}

// stopSubscriptionWorker 等待在途消息退出后再释放存储连接。
func (a *App) stopSubscriptionWorker() {
	a.subscriptionWorkerMu.Lock()
	cancel := a.subscriptionWorkerCancel
	a.subscriptionWorkerMu.Unlock()
	if cancel != nil {
		cancel()
		a.subscriptionWorkerWG.Wait()
	}
}

// claimSubscriptionDelivery 通过行锁领取单任务；发送中崩溃只能标记未知，准备中租约过期可安全恢复。
func (a *App) claimSubscriptionDelivery(ctx context.Context) (subscriptionStoredDelivery, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return subscriptionStoredDelivery{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE subscription_deliveries SET data=data||jsonb_build_object('status','uncertain','reason','发送阶段中断，微信结果未知，不得盲重发','updated_at',$1::text) WHERE data->>'status'='sending' AND (data->>'lease_until')::timestamptz<NOW()`, now()); err != nil {
		return subscriptionStoredDelivery{}, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT data FROM subscription_deliveries WHERE data->>'status'='pending' OR (data->>'status'='preparing' AND (data->>'lease_until')::timestamptz<NOW()) ORDER BY data->>'created_at',id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&raw)
	if missing(err) {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return subscriptionStoredDelivery{}, commitErr
		}
		return subscriptionStoredDelivery{}, err
	}
	if err != nil {
		return subscriptionStoredDelivery{}, err
	}
	var d subscriptionStoredDelivery
	if err = json.Unmarshal(raw, &d); err != nil {
		return d, err
	}
	d.Status = "preparing"
	d.LeaseToken = newID()
	d.LeaseUntil = time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano)
	d.UpdatedAt = now()
	if err = putRaw(ctx, tx, "subscription_deliveries", d.ID, d); err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}

// subscriptionFieldValue 依据微信字段类型规范化固定业务摘要，拒绝缺失金额或不合法标识而非编造数据。
func subscriptionFieldValue(key, value string) (string, error) {
	value = strings.TrimSpace(value)
	prefix := strings.TrimRight(key, "0123456789")
	limit := 20
	switch prefix {
	case "phrase":
		limit = 5
	case "character_string":
		limit = 32
		if !regexp.MustCompile(`^[A-Za-z0-9_\-]+$`).MatchString(value) {
			return "", fmt.Errorf("模板字符字段与来源不匹配")
		}
	case "amount":
		if !regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?元?$`).MatchString(value) {
			return "", fmt.Errorf("金额字段缺少有效业务金额")
		}
		limit = 32
	case "number":
		if !regexp.MustCompile(`^[0-9]{1,32}$`).MatchString(value) {
			return "", fmt.Errorf("数量字段无效")
		}
		limit = 32
	case "time":
		if _, err := time.Parse("2006-01-02 15:04:05", value); err != nil {
			return "", fmt.Errorf("时间字段无效")
		}
		limit = 32
	case "date":
		if len(value) >= 10 {
			value = value[:10]
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return "", fmt.Errorf("日期字段无效")
		}
		limit = 32
	}
	if value == "" {
		return "", fmt.Errorf("模板来源没有对应业务值")
	}
	runes := []rune(value)
	if len(runes) > limit {
		value = string(runes[:limit])
	}
	return value, nil
}

// subscriptionPage 限制微信落地为项目内已存在业务页，缺失或非法路径回到消息中心。
func subscriptionPage(path string) string {
	u, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil || u.IsAbs() || u.Host != "" {
		return "pages/account/messages/index"
	}
	allowed := map[string]bool{"pages/orders/detail": true, "pages/account/wallet": true, "pages/account/messages/index": true, "pages/account/profile": true, "pages/services/publish": true, "pages/services/detail": true, "pages/community/clubs/detail": true, "pages/community/clubs/edit": true, "pages/community/clubs/console": true, "pages/manage/moderation": true, "pages/manage/orders": true, "pages/manage/wallet": true, "pages/manage/clubs": true, "pages/manage/index": true, "pages/main/index": true}
	if !allowed[u.Path] || len(path) > 512 {
		return "pages/account/messages/index"
	}
	return u.String()
}

// subscriptionRecipientAllowed 在发送前复核目标订单或俱乐部归属，降权和负责人变更后旧队列不会继续泄露业务提醒。
func subscriptionRecipientAllowed(ctx context.Context, q querier, d subscriptionStoredDelivery) (bool, error) {
	allowed, err := subscriptionSceneAllowed(ctx, q, d.UserID, d.Scene)
	if err != nil || !allowed {
		return allowed, err
	}
	if d.Scene == "admin_club" {
		return true, nil
	}
	target, err := url.Parse(d.ActionPath)
	if err != nil {
		return false, nil
	}
	id := target.Query().Get("id")
	if id == "" {
		return false, nil
	}
	if d.Scene == "club_member" {
		c, e := get[Club](ctx, q, "clubs", id, false)
		if missing(e) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		return isClubManagementRecipient(ctx, q, d.UserID, c)
	}
	o, e := get[Order](ctx, q, "orders", id, false)
	if missing(e) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	switch d.Scene {
	case "provider_order", "provider_refund":
		return o.ProviderID == d.UserID, nil
	case "buyer_accepted":
		return o.BuyerID == d.UserID, nil
	case "club_dispute":
		c, e := get[Club](ctx, q, "clubs", o.ClubID, false)
		if missing(e) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		return isClubManagementRecipient(ctx, q, d.UserID, c)
	}
	return false, nil
}

// subscriptionBusinessPending 在消费一次性候选前确认待办仍有效，已取消、已撤回和已审核事项不再发送过时提醒。
func subscriptionBusinessPending(ctx context.Context, q querier, d subscriptionStoredDelivery) (bool, error) {
	target, err := url.Parse(d.ActionPath)
	if err != nil {
		return false, nil
	}
	id := target.Query().Get("id")
	if id == "" {
		return false, nil
	}
	if d.Scene == "admin_club" {
		c, e := get[Club](ctx, q, "clubs", id, false)
		if missing(e) {
			return false, nil
		}
		return e == nil && c.Status == "pending", e
	}
	if d.Scene == "club_member" {
		userID := target.Query().Get("member_id")
		if userID == "" {
			return false, nil
		}
		member, e := get[ClubMember](ctx, q, "members", id+":"+userID, false)
		if missing(e) {
			return false, nil
		}
		return e == nil && member.ClubID == id && member.UserID == userID && member.Status == "pending", e
	}
	o, e := get[Order](ctx, q, "orders", id, false)
	if missing(e) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	if o.PaymentStatus != "paid" {
		return false, nil
	}
	pendingRefund := contains([]string{"requested", "escalated", "processing"}, o.RefundStatus)
	switch d.Scene {
	case "provider_order":
		return o.Status == "open" && !pendingRefund, nil
	case "provider_refund":
		return o.RefundStatus == "requested" && !contains([]string{"cancelled", "refunded"}, o.Status), nil
	case "buyer_accepted":
		return contains([]string{"accepted", "in_progress"}, o.Status) && !pendingRefund, nil
	case "club_dispute":
		return o.RefundStatus == "escalated", nil
	}
	return false, nil
}

// prepareSubscriptionSend 在短事务内检查配置、本人偏好和微信身份并消费一个模板候选；不占业务与资金锁。
func (a *App) prepareSubscriptionSend(ctx context.Context, d subscriptionStoredDelivery) (subscriptionStoredDelivery, string, map[string]any, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return d, "", nil, err
	}
	defer tx.Rollback(ctx)
	current, err := get[subscriptionStoredDelivery](ctx, tx, "subscription_deliveries", d.ID, true)
	if err != nil {
		return d, "", nil, err
	}
	if current.Status != "preparing" || current.LeaseToken != d.LeaseToken {
		return d, "", nil, fail(409, "提醒已由其他消费者处理")
	}
	d = current
	status, reason := "", ""
	end, expiryErr := time.Parse(time.RFC3339Nano, d.ExpiresAt)
	if expiryErr != nil || !end.After(time.Now()) {
		status, reason = "expired", "业务提醒已过期"
	}
	s, err := readSubscriptionSettings(ctx, tx)
	if err != nil {
		return d, "", nil, err
	}
	c := subscriptionSceneConfig(s, d.Scene)
	if status == "" && (!s.Enabled || !c.Enabled) {
		status, reason = "skipped_disabled", "该提醒场景已关闭"
	}
	if status == "" && (!subscriptionTemplatePattern.MatchString(c.TemplateID) || len(c.Fields) == 0 || a.Config.WechatAppID == "" || a.Config.WechatSecret == "") {
		status, reason = "skipped_config", "微信服务或模板尚未配置"
	}
	p, err := subscriptionPreferences(ctx, tx, d.UserID)
	if err != nil {
		return d, "", nil, err
	}
	if status == "" && !p[d.Scene] {
		status, reason = "skipped_preference", "用户已关闭该场景提醒"
	}
	u, userErr := get[User](ctx, tx, "users", d.UserID, false)
	if userErr != nil && !missing(userErr) {
		return d, "", nil, userErr
	}
	if status == "" && (missing(userErr) || u.Status != "active") {
		status, reason = "skipped_identity", "账号不存在或已停用"
	}
	if status == "" {
		allowed, roleErr := subscriptionRecipientAllowed(ctx, tx, d)
		if roleErr != nil {
			return d, "", nil, roleErr
		}
		if !allowed {
			status, reason = "skipped_identity", "当前身份已无权接收该订单或组织的提醒"
		}
	}
	if status == "" {
		pending, pendingErr := subscriptionBusinessPending(ctx, tx, d)
		if pendingErr != nil {
			return d, "", nil, pendingErr
		}
		if !pending {
			status, reason = "expired", "业务已处理或状态已变化，待办提醒已失效"
		}
	}
	identity, identityErr := get[map[string]string](ctx, tx, "identities", d.UserID, false)
	if identityErr != nil && !missing(identityErr) {
		return d, "", nil, identityErr
	}
	if status == "" && (identity["openid"] == "" || identity["appid"] != a.Config.WechatAppID) {
		status, reason = "skipped_identity", "未绑定当前小程序的官方微信身份"
	}
	if status == "" {
		if err = a.populateSubscriptionValues(ctx, tx, &d); err != nil {
			return d, "", nil, err
		}
	}
	fields := map[string]any{}
	if status == "" {
		for key, source := range c.Fields {
			value, e := subscriptionFieldValue(key, d.Values[source])
			if e != nil {
				status, reason = "skipped_config", e.Error()
				break
			}
			fields[key] = map[string]string{"value": value}
		}
	}
	if status == "" {
		var raw []byte
		err = tx.QueryRow(ctx, `SELECT data FROM subscription_authorizations a WHERE data->>'user_id'=$1 AND data->>'appid'=$2 AND data->>'template_id'=$3 AND data->>'status'='accept' AND (COALESCE(data->>'consumed_by','')='' OR data->>'consumed_by'=$4) AND (data->>'expires_at')::timestamptz>NOW() AND NOT EXISTS(SELECT 1 FROM subscription_authorizations revoked WHERE revoked.data->>'user_id'=$1 AND revoked.data->>'appid'=$2 AND revoked.data->>'template_id'=$3 AND revoked.data->>'status' IN ('reject','ban') AND (revoked.data->>'created_at')::timestamptz >= CASE WHEN revoked.data->>'source'='wechat_callback' THEN date_trunc('second',(a.data->>'created_at')::timestamptz) ELSE (a.data->>'created_at')::timestamptz END) ORDER BY data->>'created_at',id LIMIT 1 FOR UPDATE SKIP LOCKED`, d.UserID, a.Config.WechatAppID, c.TemplateID, d.ID).Scan(&raw)
		if missing(err) {
			status, reason = "skipped_no_authorization", "没有可用的一次性订阅候选，真实权限由微信判定"
		} else if err != nil {
			return d, "", nil, err
		} else {
			var auth subscriptionAuthorization
			if err = json.Unmarshal(raw, &auth); err != nil {
				return d, "", nil, err
			}
			auth.ConsumedBy = d.ID
			d.AuthorizationID = auth.ID
			if err = putRaw(ctx, tx, "subscription_authorizations", auth.ID, auth); err != nil {
				return d, "", nil, err
			}
		}
	}
	if status != "" && d.AuthorizationID != "" {
		if _, err = tx.Exec(ctx, `UPDATE subscription_authorizations SET data=jsonb_set(data,'{consumed_by}','""'::jsonb) WHERE id=$1 AND data->>'consumed_by'=$2`, d.AuthorizationID, d.ID); err != nil {
			return d, "", nil, err
		}
		d.AuthorizationID = ""
	}
	d.TemplateID = c.TemplateID
	if status != "" {
		d.Status = status
		d.Reason = reason
	}
	d.UpdatedAt = now()
	if err = putRaw(ctx, tx, "subscription_deliveries", d.ID, d); err != nil {
		return d, "", nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return d, "", nil, err
	}
	return d, identity["openid"], fields, nil
}

// finishSubscriptionDelivery 只用当前领取令牌更新状态，过期消费者不能覆盖新管理员处理。
func (a *App) finishSubscriptionDelivery(ctx context.Context, d subscriptionStoredDelivery, status, reason string, code int) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	current, err := get[subscriptionStoredDelivery](ctx, tx, "subscription_deliveries", d.ID, true)
	if err != nil {
		return err
	}
	if current.LeaseToken != d.LeaseToken || (current.Status != "preparing" && current.Status != "sending") {
		return fail(409, "提醒处理租约已失效")
	}
	d.Status = status
	d.Reason = reason
	d.ErrorCode = code
	d.UpdatedAt = now()
	if status == "sent" {
		d.SentAt = now()
	}
	if (status == "failed_retryable" || status == "expired" && current.Status == "preparing") && d.AuthorizationID != "" {
		if _, err = tx.Exec(ctx, `UPDATE subscription_authorizations SET data=jsonb_set(data,'{consumed_by}','""'::jsonb) WHERE id=$1 AND data->>'consumed_by'=$2`, d.AuthorizationID, d.ID); err != nil {
			return err
		}
		d.AuthorizationID = ""
	}
	if err = putRaw(ctx, tx, "subscription_deliveries", d.ID, d); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// processSubscriptionOne 完成一次发送，发送前提交状态；网络中断或不完整响应保留未知而非重试。
func (a *App) processSubscriptionOne(ctx context.Context) (bool, error) {
	d, err := a.claimSubscriptionDelivery(ctx)
	if missing(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	d, openid, fields, err := a.prepareSubscriptionSend(ctx, d)
	if err != nil {
		return true, err
	}
	if d.Status != "preparing" {
		return true, nil
	}
	token, err := a.wechatAccessToken(ctx, false)
	if err != nil {
		return true, a.finishSubscriptionDelivery(ctx, d, "failed_retryable", "获取微信凭证失败，尚未发送", 0)
	}
	pending, pendingErr := subscriptionBusinessPending(ctx, a.DB, d)
	if pendingErr != nil {
		return true, a.finishSubscriptionDelivery(ctx, d, "failed_retryable", "读取最新业务状态失败，尚未发送", 0)
	}
	if !pending {
		return true, a.finishSubscriptionDelivery(ctx, d, "expired", "业务已处理或状态已变化，待办提醒已失效", 0)
	}
	input := map[string]any{"touser": openid, "template_id": d.TemplateID, "page": subscriptionPage(d.ActionPath), "miniprogram_state": "formal", "lang": "zh_CN", "data": fields}
	raw, err := json.Marshal(input)
	if err != nil {
		return true, err
	}
	d.Attempts++
	d.Status = "sending"
	d.UpdatedAt = now()
	if err = a.finishSubscriptionDelivery(ctx, d, "sending", "正在等待微信返回", 0); err != nil {
		return true, err
	}
	sendCtx, sendCancel := context.WithTimeout(ctx, 12*time.Second)
	defer sendCancel()
	req, err := http.NewRequestWithContext(sendCtx, "POST", "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token="+url.QueryEscape(token), bytes.NewReader(raw))
	if err != nil {
		return true, a.finishSubscriptionDelivery(ctx, d, "failed_retryable", "请求尚未发出", 0)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.HTTPClient == nil {
		return true, a.finishSubscriptionDelivery(ctx, d, "failed_retryable", "微信客户端尚未初始化，未发送", 0)
	}
	client := *a.HTTPClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return true, a.finishSubscriptionDelivery(ctx, d, "uncertain", "网络结果未知，不得盲目重发", 0)
	}
	defer res.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(res.Body, 65537))
	var result struct {
		ErrCode   *int   `json:"errcode"`
		MessageID string `json:"msgid"`
	}
	if readErr != nil || len(body) > 65536 || res.StatusCode != 200 || json.Unmarshal(body, &result) != nil || result.ErrCode == nil {
		return true, a.finishSubscriptionDelivery(ctx, d, "uncertain", "微信响应不完整，结果未知，不得盲目重发", 0)
	}
	code := *result.ErrCode
	if code == 0 {
		d.MessageID = result.MessageID
		return true, a.finishSubscriptionDelivery(ctx, d, "sent", "微信已接受发送请求，终端展示由微信决定", 0)
	}
	status, reason := "failed_final", "微信明确拒绝发送，请核对模板和业务字段"
	switch code {
	case 43101:
		status, reason = "skipped_no_authorization", "微信未授权该模板或订阅次数已用尽"
	case 40001, 40014, 42001:
		status, reason = "failed_retryable", "微信凭证失效，官方明确未发送，请更新凭证后重试"
		_, _ = a.wechatAccessToken(ctx, true)
	case 40037, 47003, 41030, 45009, 45011, 45015:
		status, reason = "failed_retryable", "微信明确未发送，请修正模板、落地页或频率后重试"
	}
	return true, a.finishSubscriptionDelivery(ctx, d, status, reason, code)
}
