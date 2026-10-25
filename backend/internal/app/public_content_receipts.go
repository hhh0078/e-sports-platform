// Package app 保存账号与公告版本绑定的已读记录，访客仍由客户端按版本记录。
package app

import (
	"net/http"
	"strconv"
)

// announcementState 查询当前账号是否读过指定公告版本，不在读取配置时提前标记已读。
func (a *App) announcementState(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	v, err := readPublicContent(r.Context(), a.DB)
	if err != nil {
		return nil, err
	}
	version := v.Announcement.Version
	if raw := r.URL.Query().Get("version"); raw != "" {
		version, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || version < 0 || version != v.Announcement.Version {
			return nil, fail(409, "公告已更新，请重新读取公共信息")
		}
	}
	id := reviewHash("home\n" + u.ID + "\n" + strconv.FormatInt(version, 10))
	var seen bool
	if err = a.DB.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM announcement_receipts WHERE id=$1)", id).Scan(&seen); err != nil {
		return nil, err
	}
	w.Header().Set("Cache-Control", "no-store")
	return map[string]any{"id": "home", "version": version, "seen": seen}, nil
}

// acknowledgeAnnouncement 在真正展示后幂等记录本人当前版本，拒绝旧版本误确认新公告。
func (a *App) acknowledgeAnnouncement(w http.ResponseWriter, r *http.Request) (any, error) {
	u, err := a.actor(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(8173081)"); err != nil {
		return nil, err
	}
	v, err := readPublicContent(r.Context(), tx)
	if err != nil {
		return nil, err
	}
	if in.ID != "home" || in.Version <= 0 || in.Version != v.Announcement.Version {
		return nil, fail(409, "公告版本已变化，请重新读取")
	}
	id := reviewHash("home\n" + u.ID + "\n" + strconv.FormatInt(in.Version, 10))
	if err = seedInsert(r.Context(), tx, "announcement_receipts", id, map[string]any{"user_id": u.ID, "announcement_id": "home", "version": in.Version, "read_at": now()}); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, tx.Commit(r.Context())
}
