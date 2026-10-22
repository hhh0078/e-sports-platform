// Package app 保存用户未读系统通知，已读后仅删除本人服务器副本，不影响订单聊天证据。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// InboxMessage 是服务器未读通知，本机已读历史由客户端按账号持久化。
type InboxMessage struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// inbox 返回本人未读通知及总数，稳定排序便于分页同步后再确认删除。
func (a *App) inbox(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	limit, offset := pageLimit(r)
	items, err := list[InboxMessage](r.Context(), a.DB, "inbox", "WHERE data->>'user_id'=$1 ORDER BY data->>'created_at' DESC,id DESC LIMIT $2 OFFSET $3", u.ID, limit, offset)
	if err != nil {
		return nil, err
	}
	var total int
	if err = a.DB.QueryRow(r.Context(), "SELECT count(*) FROM inbox WHERE data->>'user_id'=$1", u.ID).Scan(&total); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total}, nil
}

// acknowledgeInbox 幂等删除本人已读服务器副本，其他账号的消息不会被读取或删除。
func (a *App) acknowledgeInbox(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	if len(r.PathValue("id")) > 256 {
		return nil, fail(400, "消息标识无效")
	}
	_, err = a.DB.Exec(r.Context(), "DELETE FROM inbox WHERE id=$1 AND data->>'user_id'=$2", r.PathValue("id"), u.ID)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

// sendAdminInbox 将管理员通知原子投递给指定用户或全部活跃账号并保留审计。
func (a *App) sendAdminInbox(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	if err = a.throttle(r.Context(), "admin-notice", u.ID, 10); err != nil {
		return nil, err
	}
	var in struct {
		RecipientID string `json:"recipient_id"`
		Title       string `json:"title"`
		Body        string `json:"body"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	in.RecipientID = strings.TrimSpace(in.RecipientID)
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	if !length(in.Title, 1, 60) || !length(in.Body, 1, 5000) || len(in.RecipientID) > 128 || strings.HasPrefix(in.RecipientID, "demo_") {
		return nil, fail(400, "通知标题1至60字、正文1至5000字，接收人需为真实用户")
	}
	if in.RecipientID != "" {
		recipient, e := get[User](r.Context(), a.DB, "users", in.RecipientID, false)
		if e != nil || recipient.Status != "active" {
			return nil, fail(404, "接收用户不存在或已停用")
		}
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	prefix, stamp := newID()+"_", now()
	result, err := tx.Exec(r.Context(), `INSERT INTO inbox(id,data) SELECT $1||id,jsonb_build_object('id',$1||id,'user_id',id,'title',$2::text,'body',$3::text,'created_at',$4::text) FROM users WHERE data->>'status'='active' AND id NOT LIKE 'demo\_%' ESCAPE '\' AND ($5='' OR id=$5)`, prefix, in.Title, in.Body, stamp, in.RecipientID)
	if err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "inbox.send", in.RecipientID, "发送平台系统通知"); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return map[string]int64{"recipient_count": result.RowsAffected()}, nil
}

// sendModerationNotice 使用原因摘要作为幂等标识，确认删除后同日重复拒绝也不会无限生成通知。
func sendModerationNotice(ctx context.Context, q querier, userID, scope, reason string) error {
	stamp := now()
	sum := sha256.Sum256([]byte(userID + "\n" + scope + "\n" + reason + "\n" + stamp[:10]))
	id := "review_" + hex.EncodeToString(sum[:])
	v := InboxMessage{ID: id, UserID: userID, Title: "内容审核未通过", Body: "提交位置：" + scope + "\n原因：" + reason + "\n请修改相关文字或图片后重试。", CreatedAt: stamp}
	// 幂等标记独立于inbox保留，防止已读删除后同一天相同拒绝再次刷屏。
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `WITH claimed AS (INSERT INTO inbox_receipts(id,data) VALUES($1,jsonb_build_object('created_at',$3::text)) ON CONFLICT(id) DO NOTHING RETURNING id) INSERT INTO inbox(id,data) SELECT id,$2::jsonb FROM claimed`, id, raw, stamp)
	return err
}

// registerInboxRoutes 将系统通知与不可删除的订单沟通路由分离。
func (a *App) registerInboxRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/messages", a.wrap(a.inbox))
	mux.HandleFunc("DELETE /api/messages/{id}", a.wrap(a.acknowledgeInbox))
	mux.HandleFunc("POST /api/admin/messages", a.wrap(a.sendAdminInbox))
}
