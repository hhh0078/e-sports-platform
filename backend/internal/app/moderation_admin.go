// Package app 提供管理员字段级图文待审列表、人工决定、AI重试与鉴权原图预览。
package app

import (
	"net/http"
	"strconv"
	"strings"
)

// moderationTaskView 只向管理员返回审核内容，去除内部租约、SQL路径和摘要。
func moderationTaskView(task ModerationTask, detail bool) map[string]any {
	preview := task.Preview
	if !detail {
		runes := []rune(preview)
		if len(runes) > 160 {
			preview = string(runes[:160]) + "…"
		}
	}
	result := map[string]any{"id": task.ID, "target_type": task.TargetType, "target_id": task.TargetID, "field_code": task.FieldCode, "field_label": task.FieldLabel, "kind": task.Kind, "status": task.Status, "actor_id": task.ActorID, "owner_id": task.OwnerID, "content_version": task.ContentVersion, "preview": preview, "reason": task.Reason, "last_error": task.LastError, "decision_source": task.DecisionSource, "reviewed_by": task.ReviewedBy, "attempts": task.Attempts, "created_at": task.CreatedAt, "updated_at": task.UpdatedAt}
	if task.Kind == "image" {
		result["image_preview_url"] = "/api/admin/moderation/" + task.ID + "/image"
	}
	return result
}

// adminModeration 返回稳定ID游标分页和完整状态计数，不暴露任何商户或AI配置。
func (a *App) adminModeration(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "all" && !contains(reviewStatuses, status) {
		return nil, fail(400, "审核状态无效")
	}
	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			return nil, fail(400, "每页数量须为1至100")
		}
		limit = n
	}
	args := []any{}
	conditions := []string{}
	if status != "" && status != "all" {
		args = append(args, status)
		conditions = append(conditions, "data->>'status'=$"+strconv.Itoa(len(args)))
	}
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		anchor, err := get[ModerationTask](r.Context(), a.DB, "moderation_tasks", cursor, false)
		if err != nil {
			return nil, fail(400, "审核分页游标无效")
		}
		args = append(args, anchor.CreatedAt, anchor.ID)
		conditions = append(conditions, "(data->>'created_at',id)<($"+strconv.Itoa(len(args)-1)+",$"+strconv.Itoa(len(args))+")")
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	args = append(args, limit+1)
	tasks, err := list[ModerationTask](r.Context(), a.DB, "moderation_tasks", where+" ORDER BY data->>'created_at' DESC,id DESC LIMIT $"+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	next := ""
	if len(tasks) > limit {
		tasks = tasks[:limit]
		next = tasks[len(tasks)-1].ID
	}
	items := []map[string]any{}
	for _, task := range tasks {
		items = append(items, moderationTaskView(task, false))
	}
	counts := map[string]int64{"total": 0}
	for _, key := range reviewStatuses {
		counts[key] = 0
	}
	rows, err := a.DB.Query(r.Context(), "SELECT data->>'status',count(*) FROM moderation_tasks GROUP BY data->>'status'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int64
		if err = rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		counts[key] = count
		counts["total"] += count
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "next_cursor": next, "counts": counts}, nil
}

// adminModerationDetail 返回管理员所选任务完整文字，所有读取仍重新校验管理员身份。
func (a *App) adminModerationDetail(w http.ResponseWriter, r *http.Request) (any, error) {
	if _, err := a.admin(r); err != nil {
		return nil, err
	}
	task, err := get[ModerationTask](r.Context(), a.DB, "moderation_tasks", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	return moderationTaskView(task, true), nil
}

// adminModerationDecision 执行单次人工决定，AI迟到回包不能覆盖已完成结果。
func (a *App) adminModerationDecision(w http.ResponseWriter, r *http.Request) (any, error) {
	_, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err = decode(r, &in); err != nil {
		return nil, err
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !contains([]string{"allow", "reject"}, in.Decision) || !length(in.Reason, 0, 500) || in.Decision == "reject" && in.Reason == "" {
		return nil, fail(400, "请选择通过或拒绝，拒绝时填写1至500字原因")
	}
	task, err := get[ModerationTask](r.Context(), a.DB, "moderation_tasks", r.PathValue("id"), false)
	if err != nil {
		return nil, err
	}
	if err = a.finishModeration(r.Context(), task, in.Decision, in.Reason, "manual", ""); err != nil {
		return nil, err
	}
	task, err = get[ModerationTask](r.Context(), a.DB, "moderation_tasks", task.ID, false)
	if err != nil {
		return nil, err
	}
	return moderationTaskView(task, true), nil
}

// retryModerationTask 将仍有效的人工待审任务交回AI，无可用配置时保留人工状态。
func (a *App) retryModerationTask(w http.ResponseWriter, r *http.Request) (any, error) {
	actor, err := a.admin(r)
	if err != nil {
		return nil, err
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(r.Context())
	task, err := get[ModerationTask](r.Context(), tx, "moderation_tasks", r.PathValue("id"), true)
	if err != nil {
		return nil, err
	}
	if task.Status != "manual_pending" {
		return nil, fail(409, "仅人工待审任务可重新提交AI")
	}
	valid, checkErr := a.isReviewTaskCurrent(r.Context(), task)
	if checkErr != nil {
		return nil, checkErr
	}
	if !valid {
		task.Status = "superseded"
		task.Reason = "内容已更新"
	} else {
		cfg, e := a.readAISettings(r.Context(), tx)
		if e == nil && cfg.APIKey != "" && validateAISettings(cfg) == nil {
			task.Status = "ai_pending"
			task.LastError = ""
			// 图片字段复用源图审核，重试业务字段时也唤醒仍待人工的当前源图任务。
			if task.Kind == "image" && task.TargetType != "evidence" {
				key := moderationTargetID("evidence", task.MediaID, "image")
				if _, err = tx.Exec(r.Context(), `UPDATE moderation_tasks SET data=jsonb_set(jsonb_set(data,'{status}','"ai_pending"'::jsonb,true),'{last_error}','""'::jsonb,true) WHERE data->>'status'='manual_pending' AND id=(SELECT data->>'task_id' FROM moderation_targets WHERE id=$1)`, key); err != nil {
					return nil, err
				}
			}
		} else {
			task.LastError = "未配置可用AI，继续等待人工审核"
		}
	}
	task.UpdatedAt = now()
	if err = putRaw(r.Context(), tx, "moderation_tasks", task.ID, task); err != nil {
		return nil, err
	}
	if err = audit(r.Context(), tx, actor.ID, "moderation.retry", task.ID, task.Status); err != nil {
		return nil, err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return nil, err
	}
	return moderationTaskView(task, true), nil
}

// moderationImagePreview 只向当前平台管理员返回原图；普通链接和其他角色无法读取隔离证据。
func (a *App) moderationImagePreview(w http.ResponseWriter, r *http.Request) {
	if _, err := a.admin(r); err != nil {
		status := 403
		if ae, ok := err.(*apiError); ok {
			status = ae.Status
		}
		respond(w, status, map[string]string{"error": "仅平台管理员可查看审核原图"})
		return
	}
	task, err := get[ModerationTask](r.Context(), a.DB, "moderation_tasks", r.PathValue("id"), false)
	if err != nil || task.Kind != "image" {
		respond(w, 404, map[string]string{"error": "审核图片不存在"})
		return
	}
	media, err := get[Evidence](r.Context(), a.DB, "evidence", task.MediaID, false)
	if err != nil || len(media.Data) == 0 {
		a.rejectedMedia(w, r)
		return
	}
	w.Header().Set("Content-Type", media.MIME)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Write(media.Data)
}

// registerModerationRoutes 注册独立审核模块，避免通用后台读取原始审核任务或私密证据。
func (a *App) registerModerationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/moderation", a.wrap(a.adminModeration))
	mux.HandleFunc("GET /api/admin/moderation/{id}", a.wrap(a.adminModerationDetail))
	mux.HandleFunc("POST /api/admin/moderation/{id}/decision", a.wrap(a.adminModerationDecision))
	mux.HandleFunc("POST /api/admin/moderation/{id}/retry", a.wrap(a.retryModerationTask))
	mux.HandleFunc("GET /api/admin/moderation/{id}/image", a.moderationImagePreview)
	mux.HandleFunc("GET /api/media/moderation-rejected.png", a.rejectedMedia)
}
