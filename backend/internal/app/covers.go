// Package app 提供管理员原创封面上传及公开图片读取，与订单私密证据严格隔离。
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
)

// uploadCover 只允许管理员把经过图片校验的文件发布为游戏封面。
func (a *App) uploadCover(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	if err = a.throttle(r.Context(), "cover", u.ID, 30); err != nil {
		return nil, err
	}
	raw, mime, err := parseImage(w, r)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	v := Evidence{ID: newID(), OwnerID: u.ID, SHA256: hex.EncodeToString(sum[:]), MIME: mime, Data: raw, CreatedAt: now(), Public: true, Moderation: "trusted_platform"}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if err = put(r.Context(), tx, "evidence", v.ID, v); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, u.ID, "cover.upload", v.ID, v.SHA256); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return map[string]string{"url": "/api/media/" + v.ID, "sha256": v.SHA256}, nil
}

// viewCover 仅公开管理员主动发布的封面，私密证据不能通过此入口访问。
func (a *App) viewCover(w http.ResponseWriter, r *http.Request) {
	v, err := get[Evidence](r.Context(), a.DB, "evidence", r.PathValue("id"), false)
	if err != nil || !v.Public {
		respond(w, 404, map[string]string{"error": "封面不存在"})
		return
	}
	w.Header().Set("Content-Type", v.MIME)
	w.Header().Set("Content-Length", strconv.Itoa(len(v.Data)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Write(v.Data)
}
