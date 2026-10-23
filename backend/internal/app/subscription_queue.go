// Package app 以事务发件箱保存站内通知和微信发送任务，读后删除站内消息不影响业务幂等。
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DomainNotice 是可信业务事件，仅固定服务端调用能创建，不能作为公开任意推送接口。
type DomainNotice struct {
	ID          string
	Scene       string
	Event       string
	UserIDs     []string
	Admins      bool
	Title       string
	Body        string
	ActionPath  string
	ActionLabel string
	Values      map[string]string
	SkipInbox   bool
}

// SubscriptionDelivery 保存独立于站内消息生命周期的发送事实与安全失败原因。
type SubscriptionDelivery struct {
	ID              string            `json:"id"`
	UserID          string            `json:"user_id"`
	Scene           string            `json:"scene"`
	Event           string            `json:"event"`
	Title           string            `json:"title"`
	Status          string            `json:"status"`
	Reason          string            `json:"reason"`
	ErrorCode       int               `json:"error_code"`
	Attempts        int               `json:"attempts"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
	SentAt          string            `json:"sent_at,omitempty"`
	ExpiresAt       string            `json:"expires_at"`
	CanRetry        bool              `json:"can_retry"`
	TemplateID      string            `json:"template_id,omitempty"`
	ActionPath      string            `json:"-"`
	Values          map[string]string `json:"-"`
	LeaseToken      string            `json:"-"`
	LeaseUntil      string            `json:"-"`
	AuthorizationID string            `json:"-"`
	MessageID       string            `json:"-"`
}

// subscriptionStoredDelivery 保持内部字段可持久化，同时对管理员API另行投影隐藏。
type subscriptionStoredDelivery struct {
	SubscriptionDelivery
	ActionPath      string            `json:"action_path"`
	Values          map[string]string `json:"values"`
	LeaseToken      string            `json:"lease_token"`
	LeaseUntil      string            `json:"lease_until"`
	AuthorizationID string            `json:"authorization_id"`
	MessageID       string            `json:"message_id"`
}

// queueDomainNotice 将单事件每接收人的站内消息与微信候选原子写入；事务调用者共同提交，池调用者自建事务。
func queueDomainNotice(ctx context.Context, q querier, n DomainNotice) error {
	if pool, ok := q.(*pgxpool.Pool); ok {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err = queueDomainNotice(ctx, tx, n); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if n.ID == "" || !validSubscriptionScene(n.Scene) || !validSubscriptionEvent(n.Scene, n.Event) || !length(n.Title, 1, 60) || !length(n.Body, 0, 5000) {
		return fmt.Errorf("业务提醒定义无效")
	}
	recipients := map[string]bool{}
	for _, id := range n.UserIDs {
		if id != "" && !strings.HasPrefix(id, "demo_") {
			recipients[id] = true
		}
	}
	if n.Admins {
		admins, err := list[User](ctx, q, "users", `WHERE data->>'role'='admin' AND data->>'status'='active'`)
		if err != nil {
			return err
		}
		for _, u := range admins {
			recipients[u.ID] = true
		}
	}
	stamp := now()
	for userID := range recipients {
		u, err := get[User](ctx, q, "users", userID, false)
		if missing(err) {
			continue
		}
		if err != nil {
			return err
		}
		if u.Status != "active" {
			continue
		}
		receiptID := "event_" + subscriptionHash(n.ID, n.Scene, userID)
		var claimed string
		err = q.QueryRow(ctx, `INSERT INTO subscription_receipts(id,data) VALUES($1,jsonb_build_object('created_at',$2::text)) ON CONFLICT(id) DO NOTHING RETURNING id`, receiptID, stamp).Scan(&claimed)
		if missing(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !n.SkipInbox {
			m := InboxMessage{ID: receiptID, UserID: userID, Title: n.Title, Body: n.Body, CreatedAt: stamp, ActionPath: n.ActionPath, ActionLabel: n.ActionLabel}
			if err = putRaw(ctx, q, "inbox", m.ID, m); err != nil {
				return err
			}
		}
		id := "delivery_" + subscriptionHash(n.ID, n.Scene, userID)
		values := map[string]string{"title": n.Title, "summary": n.Title, "time": time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02 15:04:05"), "count": "1"}
		for _, source := range subscriptionSources {
			if v, ok := n.Values[source]; ok {
				values[source] = v
			}
		}
		d := subscriptionStoredDelivery{SubscriptionDelivery: SubscriptionDelivery{ID: id, UserID: userID, Scene: n.Scene, Event: n.Event, Title: n.Title, Status: "pending", CreatedAt: stamp, UpdatedAt: stamp, ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)}, ActionPath: n.ActionPath, Values: values}
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		_, err = q.Exec(ctx, `INSERT INTO subscription_deliveries(id,data) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET data=CASE WHEN subscription_deliveries.data->>'status'='pending' THEN jsonb_set(subscription_deliveries.data,'{values,count}',to_jsonb(((COALESCE(subscription_deliveries.data->'values'->>'count','1'))::integer+1)::text)) ELSE subscription_deliveries.data END`, id, raw)
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateSubscriptions 建立领取和授权索引，并给所有新增持久字段标明中文用途。
func migrateSubscriptions(ctx context.Context, q querier) error {
	_, err := q.Exec(ctx, `-- 微信订阅消息迁移：不回填历史事件、不改变业务和资金事实。
CREATE INDEX IF NOT EXISTS subscription_pending_idx ON subscription_deliveries ((data->>'status'),(data->>'created_at'),id);
CREATE INDEX IF NOT EXISTS subscription_user_idx ON subscription_deliveries ((data->>'user_id'),(data->>'created_at'));
CREATE INDEX IF NOT EXISTS subscription_authorization_idx ON subscription_authorizations ((data->>'user_id'),(data->>'template_id'),(data->>'created_at'));
COMMENT ON COLUMN subscription_deliveries.data IS '业务提醒发件箱：用户、场景、固定摘要、模板快照、领取租约和已发送或未知结果，不含OpenID和凭证';
COMMENT ON COLUMN subscription_authorizations.data IS '一次显式操作的模板级候选与消费事实；客户端报告不代表微信最终权限，不保存OpenID';
COMMENT ON COLUMN subscription_preferences.data IS '本人固定场景提醒偏好，关闭不影响站内消息和业务';
COMMENT ON COLUMN subscription_receipts.data IS '永久业务事件与授权请求的幂等摘要，独立于已读消息删除';
COMMENT ON COLUMN subscription_callbacks.data IS '已验签微信订阅事件的去重摘要、模板与结果，不保存原始私密身份和加密正文';`)
	return err
}
