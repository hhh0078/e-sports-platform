// Package app 提供平台管理员独立模板配置和发送记录管理，普通用户不能读取或触发他人通知。
package app

import (
	"net/http"
	"strings"
	"time"
)

// validateSubscriptionSettings 验证固定场景、官方字段名和来源，禁止任意接口地址或自定义敏感原文。
func validateSubscriptionSettings(s SubscriptionSettings) error {
	if len(s.Scenes) != len(subscriptionCatalog) {
		return fail(400, "请完整配置六类提醒场景")
	}
	seen := map[string]bool{}
	for _, v := range s.Scenes {
		if !validSubscriptionScene(v.Scene) || seen[v.Scene] {
			return fail(400, "提醒场景无效或重复")
		}
		seen[v.Scene] = true
		if v.TemplateID == "" && len(v.Fields) == 0 && !v.Enabled {
			continue
		}
		if !subscriptionTemplatePattern.MatchString(v.TemplateID) || len(v.Fields) < 1 || len(v.Fields) > 10 {
			return fail(400, "模板ID和字段映射不完整，未配置场景请保持关闭")
		}
		for key, source := range v.Fields {
			if !subscriptionFieldPattern.MatchString(key) {
				return fail(400, "微信模板字段名无效")
			}
			allowed := false
			for _, s := range subscriptionSources {
				if s == source {
					allowed = true
				}
			}
			if !allowed {
				return fail(400, "模板字段只能选择固定业务来源")
			}
			if strings.HasPrefix(key, "amount") && source != "amount" || strings.HasPrefix(key, "number") && source != "count" || (strings.HasPrefix(key, "time") || strings.HasPrefix(key, "date")) && source != "time" && source != "created_time" {
				return fail(400, "金额、数量和时间字段须选择对应来源")
			}
		}
	}
	return nil
}

// adminSubscriptionSettings 返回管理员模板和固定来源目录，不暴露回调Token或私密身份。
func (a *App) adminSubscriptionSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	s, err := readSubscriptionSettings(r.Context(), a.DB)
	if err != nil {
		return nil, err
	}
	return map[string]any{"enabled": s.Enabled, "scenes": s.Scenes, "sources": subscriptionSources, "scene_catalog": subscriptionCatalog}, nil
}

// saveAdminSubscriptionSettings 事务保存平台开关与模板配置并记录审计，不回填旧业务事件。
func (a *App) saveAdminSubscriptionSettings(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var s SubscriptionSettings
	if err = decode(r, &s); err != nil {
		return nil, err
	}
	for i := range s.Scenes {
		s.Scenes[i].TemplateID = strings.TrimSpace(s.Scenes[i].TemplateID)
	}
	if err = validateSubscriptionSettings(s); err != nil {
		return nil, err
	}
	if err = a.validateOfficialSubscriptionSettings(r.Context(), s); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = putRaw(r.Context(), tx, "settings", "wechat_subscriptions", s); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "subscriptions.settings", "wechat_subscriptions", "修改微信订阅模板与场景开关"); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return a.adminSubscriptionSettings(w, r)
}

// subscriptionRetryable 仅明确未送达且仍有时效的任务允许再次尝试，未知网络结果永久禁止盲重发。
func subscriptionRetryable(d subscriptionStoredDelivery) bool {
	if end, err := time.Parse(time.RFC3339Nano, d.ExpiresAt); err != nil || !end.After(time.Now()) {
		return false
	}
	switch d.Status {
	case "failed_retryable", "skipped_config", "skipped_disabled", "skipped_preference", "skipped_no_authorization", "skipped_identity":
		return true
	}
	return false
}

// adminSubscriptionDeliveries 分页展示脱敏状态，支持固定场景与发送状态筛选。
func (a *App) adminSubscriptionDeliveries(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	scene, status := r.URL.Query().Get("scene"), r.URL.Query().Get("status")
	if scene != "" && !validSubscriptionScene(scene) || len(status) > 40 {
		return nil, fail(400, "筛选条件无效")
	}
	limit, offset := pageLimit(r)
	where := `WHERE ($1='' OR data->>'scene'=$1) AND ($2='' OR data->>'status'=$2)`
	items, err := list[subscriptionStoredDelivery](r.Context(), a.DB, "subscription_deliveries", where+` ORDER BY data->>'created_at' DESC,id DESC LIMIT $3 OFFSET $4`, scene, status, limit, offset)
	if err != nil {
		return nil, err
	}
	out := []SubscriptionDelivery{}
	for _, d := range items {
		d.CanRetry = subscriptionRetryable(d)
		out = append(out, d.SubscriptionDelivery)
	}
	var total int
	if err = a.DB.QueryRow(r.Context(), "SELECT count(*) FROM subscription_deliveries "+where, scene, status).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": out, "total": total}, nil
}

// retrySubscriptionDelivery 只重排服务端已知未送达任务，并保留尝试计数及管理员审计。
func (a *App) retrySubscriptionDelivery(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	d, err := get[subscriptionStoredDelivery](r.Context(), tx, "subscription_deliveries", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if !subscriptionRetryable(d) {
		return nil, fail(409, "该任务已发送、结果未知或已过期，不能重复发送")
	}
	d.Status = "pending"
	d.Reason = "管理员重新排队"
	d.LeaseToken = ""
	d.LeaseUntil = ""
	d.AuthorizationID = ""
	d.UpdatedAt = now()
	if err = putRaw(r.Context(), tx, "subscription_deliveries", d.ID, d); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "subscriptions.retry", d.ID, "重排明确未送达的订阅消息"); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}
