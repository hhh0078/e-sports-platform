// Package app 定义微信订阅场景、用户偏好与模板配置，客户端授权报告仅作为发送候选而非永久权限。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// SubscriptionScene 定义一个固定业务场景及其微信官方模板字段映射。
type SubscriptionScene struct {
	Scene      string            `json:"scene"`
	Enabled    bool              `json:"enabled"`
	TemplateID string            `json:"template_id"`
	Fields     map[string]string `json:"fields"`
}

// SubscriptionSettings 保存运营者选定的真实微信模板；初始关闭，缺配置只影响微信提醒。
type SubscriptionSettings struct {
	Enabled bool                `json:"enabled"`
	Scenes  []SubscriptionScene `json:"scenes"`
}

// subscriptionCatalogItem 提供客户端固定场景的中文说明。
type subscriptionCatalogItem struct {
	Scene       string `json:"scene"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

var subscriptionCatalog = []subscriptionCatalogItem{
	{"provider_order", "服务新订单", "有人购买你的服务并付款后提醒"},
	{"provider_refund", "退款申请", "买家申请退款时提醒打手处理"},
	{"buyer_accepted", "打手接单", "你的订单被打手接受后提醒"},
	{"club_member", "入会申请", "有人申请加入你管理的俱乐部时提醒"},
	{"club_dispute", "俱乐部订单争议", "俱乐部挂靠打手的订单申请平台介入时提醒"},
	{"admin_club", "俱乐部待认证", "新俱乐部需要平台管理员审核时提醒"},
}
var subscriptionSources = []string{"title", "status", "amount", "time", "order_no", "target_name", "count", "summary", "service_name", "actor_name", "club_name", "created_time"}
var subscriptionTemplatePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{10,128}$`)
var subscriptionFieldPattern = regexp.MustCompile(`^(thing|phrase|character_string|amount|number|time|date)[1-9][0-9]{0,2}$`)
var subscriptionRequestPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

// subscriptionHash 将业务标识摘要为固定长度，避免原始身份进入任务主键和日志。
func subscriptionHash(parts ...string) string {
	v := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(v[:])
}

// validSubscriptionScene 判断业务场景是否在服务端固定白名单内。
func validSubscriptionScene(scene string) bool {
	for _, s := range subscriptionCatalog {
		if s.Scene == scene {
			return true
		}
	}
	return false
}

// subscriptionSceneAllowed 依据服务端当前身份显示本人可订阅场景，俱乐部和平台待办不暴露给普通用户。
func subscriptionSceneAllowed(ctx context.Context, q querier, userID, scene string) (bool, error) {
	if !validSubscriptionScene(scene) {
		return false, nil
	}
	if scene == "admin_club" {
		u, err := get[User](ctx, q, "users", userID, false)
		return err == nil && u.Status == "active" && u.Role == "admin", err
	}
	if scene == "club_member" || scene == "club_dispute" {
		var exists bool
		err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clubs WHERE data->>'owner_id'=$1)`, userID).Scan(&exists)
		return exists, err
	}
	return true, nil
}

// defaultSubscriptionSettings 返回安全关闭且包含全部场景的初始配置。
func defaultSubscriptionSettings() SubscriptionSettings {
	s := SubscriptionSettings{Scenes: []SubscriptionScene{}}
	for _, v := range subscriptionCatalog {
		s.Scenes = append(s.Scenes, SubscriptionScene{Scene: v.Scene, Fields: map[string]string{}})
	}
	return s
}

// readSubscriptionSettings 读取配置并兼容首次上线的空设置。
func readSubscriptionSettings(ctx context.Context, q querier) (SubscriptionSettings, error) {
	s, err := get[SubscriptionSettings](ctx, q, "settings", "wechat_subscriptions", false)
	if missing(err) {
		return defaultSubscriptionSettings(), nil
	}
	return s, err
}

// subscriptionSceneConfig 从固定配置中寻找指定业务场景。
func subscriptionSceneConfig(s SubscriptionSettings, scene string) SubscriptionScene {
	for _, v := range s.Scenes {
		if v.Scene == scene {
			return v
		}
	}
	return SubscriptionScene{Scene: scene, Fields: map[string]string{}}
}

// subscriptionPreferences 读取本人独立偏好，未主动关闭的场景默认开启但仍须微信授权。
func subscriptionPreferences(ctx context.Context, q querier, userID string) (map[string]bool, error) {
	v, err := get[map[string]bool](ctx, q, "subscription_preferences", userID, false)
	if missing(err) {
		v = map[string]bool{}
		err = nil
	}
	if err != nil {
		return nil, err
	}
	for _, s := range subscriptionCatalog {
		if _, ok := v[s.Scene]; !ok {
			v[s.Scene] = true
		}
	}
	return v, nil
}

// subscriptionView 组合当前账号可订阅模板，不返回后台字段映射或微信身份。
func (a *App) subscriptionView(ctx context.Context, userID string) (any, error) {
	s, err := readSubscriptionSettings(ctx, a.DB)
	if err != nil {
		return nil, err
	}
	p, err := subscriptionPreferences(ctx, a.DB, userID)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for _, v := range subscriptionCatalog {
		allowed, checkErr := subscriptionSceneAllowed(ctx, a.DB, userID, v.Scene)
		if checkErr != nil {
			return nil, checkErr
		}
		if !allowed {
			continue
		}
		c := subscriptionSceneConfig(s, v.Scene)
		reason := ""
		if !s.Enabled || !c.Enabled {
			reason = "该提醒尚未启用"
		} else if !subscriptionTemplatePattern.MatchString(c.TemplateID) || len(c.Fields) == 0 {
			reason = "管理员尚未配置可用模板"
		} else if a.Config.WechatAppID == "" || a.Config.WechatSecret == "" {
			reason = "小程序微信服务尚未配置"
		}
		items = append(items, map[string]any{"scene": v.Scene, "label": v.Label, "description": v.Description, "template_id": c.TemplateID, "enabled": c.Enabled, "available": reason == "", "reason": reason, "preference": p[v.Scene]})
	}
	return map[string]any{"enabled": s.Enabled, "scenes": items, "preferences": p, "authorization_note": "一次性订阅由微信判定，每次操作授权不代表永久通知；拒绝不会影响使用。"}, nil
}

// subscriptions 返回本人可用提醒与本地运营偏好。
func (a *App) subscriptions(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	return a.subscriptionView(r.Context(), u.ID)
}

// saveSubscriptionPreferences 合并本人白名单偏好，拒绝任意用户或模板注入。
func (a *App) saveSubscriptionPreferences(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Preferences map[string]bool `json:"preferences"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if len(in.Preferences) < 1 || len(in.Preferences) > 6 {
		return nil, fail(400, "请选择有效的提醒场景")
	}
	for scene := range in.Preferences {
		allowed, checkErr := subscriptionSceneAllowed(r.Context(), a.DB, u.ID, scene)
		if checkErr != nil {
			return nil, checkErr
		}
		if !validSubscriptionScene(scene) || !allowed {
			return nil, fail(400, "提醒场景无效")
		}
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "subscription-preferences:"+u.ID); err != nil {
		return nil, err
	}
	p, err := subscriptionPreferences(r.Context(), tx, u.ID)
	if err != nil {
		return nil, err
	}
	for k, v := range in.Preferences {
		p[k] = v
	}
	if err = putRaw(r.Context(), tx, "subscription_preferences", u.ID, p); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return a.subscriptionView(r.Context(), u.ID)
}

// subscriptionAuthorization 保存一次模板候选，消费后不会因重复上报再生，真实授权由微信最终确认。
type subscriptionAuthorization struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	AppID      string `json:"appid"`
	TemplateID string `json:"template_id"`
	Status     string `json:"status"`
	Source     string `json:"source"`
	CreatedAt  string `json:"created_at"`
	ConsumedBy string `json:"consumed_by"`
	ExpiresAt  string `json:"expires_at"`
}

// saveSubscriptionAuthorizations 记录显式点击的微信结果；同一次同模板跨场景只能产生一个候选。
func (a *App) saveSubscriptionAuthorizations(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		RequestID string `json:"request_id"`
		Results   []struct {
			Scene      string `json:"scene"`
			TemplateID string `json:"template_id"`
			Status     string `json:"status"`
		} `json:"results"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	if !subscriptionRequestPattern.MatchString(in.RequestID) || len(in.Results) < 1 || len(in.Results) > 6 {
		return nil, fail(400, "授权记录格式不正确")
	}
	settings, err := readSubscriptionSettings(r.Context(), a.DB)
	if err != nil {
		return nil, err
	}
	templates := map[string]string{}
	for _, result := range in.Results {
		c := subscriptionSceneConfig(settings, result.Scene)
		allowed, checkErr := subscriptionSceneAllowed(r.Context(), a.DB, u.ID, result.Scene)
		if checkErr != nil {
			return nil, checkErr
		}
		if !allowed {
			return nil, fail(403, "当前身份不能订阅该场景")
		}
		if !validSubscriptionScene(result.Scene) || !subscriptionTemplatePattern.MatchString(result.TemplateID) || c.TemplateID != result.TemplateID {
			return nil, fail(400, "模板与提醒场景不匹配，请刷新后重试")
		}
		if result.Status != "accept" && result.Status != "reject" && result.Status != "ban" && result.Status != "filter" && result.Status != "fail" {
			return nil, fail(400, "授权结果无效")
		}
		if status, exists := templates[result.TemplateID]; exists && status != result.Status {
			return nil, fail(400, "相同模板的授权结果不一致")
		}
		templates[result.TemplateID] = result.Status
	}
	if len(templates) > 3 {
		return nil, fail(400, "一次最多授权三个不同模板")
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "subscription-auth:"+u.ID); err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(in)
	requestKey := "request_" + subscriptionHash(u.ID, in.RequestID)
	var oldHash string
	e := tx.QueryRow(r.Context(), "SELECT data->>'hash' FROM subscription_receipts WHERE id=$1", requestKey).Scan(&oldHash)
	hash := subscriptionHash(string(payload))
	if e == nil {
		if oldHash != hash {
			return nil, fail(409, "授权记录标识已被其它内容使用")
		}
		return map[string]bool{"ok": true}, nil
	}
	if !missing(e) {
		return nil, e
	}
	if err = a.throttle(r.Context(), "subscription-auth", u.ID, 30); err != nil {
		return nil, err
	}
	var inserted string
	err = tx.QueryRow(r.Context(), `INSERT INTO subscription_receipts(id,data) VALUES($1,jsonb_build_object('hash',$2::text,'created_at',$3::text)) ON CONFLICT(id) DO NOTHING RETURNING id`, requestKey, hash, now()).Scan(&inserted)
	if missing(err) {
		return nil, fail(409, "授权记录正在处理，请稍后重试")
	}
	if err != nil {
		return nil, err
	}
	for template, status := range templates {
		v := subscriptionAuthorization{ID: subscriptionHash(u.ID, in.RequestID, template), UserID: u.ID, AppID: a.Config.WechatAppID, TemplateID: template, Status: status, Source: "client_report", CreatedAt: now(), ExpiresAt: time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339Nano)}
		if err = putRaw(r.Context(), tx, "subscription_authorizations", v.ID, v); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

// registerSubscriptionRoutes 将本人订阅与平台配置路由分开，官方回调另走验签入口。
func (a *App) registerSubscriptionRoutes(mux *http.ServeMux) {
	routes := map[string]func(http.ResponseWriter, *http.Request) (any, error){"GET /api/subscriptions": a.subscriptions, "PATCH /api/subscriptions/preferences": a.saveSubscriptionPreferences, "POST /api/subscriptions/authorizations": a.saveSubscriptionAuthorizations, "GET /api/admin/subscriptions/templates": a.subscriptionTemplates, "GET /api/admin/subscriptions/settings": a.adminSubscriptionSettings, "PATCH /api/admin/subscriptions/settings": a.saveAdminSubscriptionSettings, "GET /api/admin/subscriptions/deliveries": a.adminSubscriptionDeliveries, "POST /api/admin/subscriptions/deliveries/{id}/retry": a.retrySubscriptionDelivery}
	for route, handler := range routes {
		mux.HandleFunc(route, a.wrap(handler))
	}
	mux.HandleFunc("GET /api/wechat/messages", a.subscriptionCallback)
	mux.HandleFunc("POST /api/wechat/messages", a.subscriptionCallback)
}
