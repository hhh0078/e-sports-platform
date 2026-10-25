// Package app 在事务外调用AI，并用字段版本、租约和人工决定保护后置审核结果。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"
)

var reviewStatuses = []string{"ai_pending", "ai_processing", "manual_pending", "allowed", "rejected", "superseded"}

// migrateModeration 增加任务领取、归属查找和字段单调版本索引，不回填或改变交易金额。
func migrateModeration(ctx context.Context, q querier) error {
	_, err := q.Exec(ctx, `-- 图文审核迁移：持久化字段任务和版本；不处理历史资金和认证状态。
 CREATE INDEX IF NOT EXISTS moderation_pending_idx ON moderation_tasks ((data->>'status'),(data->>'created_at'),id);
 CREATE INDEX IF NOT EXISTS moderation_task_target_idx ON moderation_tasks ((data->>'target_type'),(data->>'target_id'),(data->>'field_code'));
 CREATE INDEX IF NOT EXISTS moderation_task_media_idx ON moderation_tasks ((data->>'media_id')) WHERE COALESCE(data->>'media_id','')<>'';
 CREATE INDEX IF NOT EXISTS moderation_target_record_idx ON moderation_targets ((data->>'target_type'),(data->>'target_id'));
 CREATE UNIQUE INDEX IF NOT EXISTS moderation_target_version_idx ON moderation_tasks ((data->>'target_type'),(data->>'target_id'),(data->>'field_code'),(data->>'content_version'));
 COMMENT ON COLUMN moderation_tasks.data IS '图文字段审核任务：含白名单类型、业务ID、字段编码、单调版本、内容摘要、归属、租约、AI或人工决定和错误；不保存支付身份秘密';
 COMMENT ON COLUMN moderation_targets.data IS '图文字段当前版本和审核状态；用于防止旧结果覆盖新内容以及交易原件的对外屏蔽';`)
	return err
}

// StartModerationWorker 启动有限并发后台处理，数据库为队列事实来源，进程重启不会丢任务。
func (a *App) StartModerationWorker(ctx context.Context) {
	a.reviewWorkerMu.Lock()
	defer a.reviewWorkerMu.Unlock()
	if a.reviewWorkerCancel != nil {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	a.reviewWorkerCancel = cancel
	for i := 0; i < 4; i++ {
		a.reviewWorkerWG.Add(1)
		go func() {
			defer a.reviewWorkerWG.Done()
			ticker := time.NewTicker(750 * time.Millisecond)
			defer ticker.Stop()
			for {
				if _, err := a.processModerationOne(workerCtx); err != nil && workerCtx.Err() == nil {
					log.Printf("图文后台审核暂缓：%v", err)
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

// stopModerationWorker 等待在途审核释放连接，再关闭数据库。
func (a *App) stopModerationWorker() {
	a.reviewWorkerMu.Lock()
	cancel := a.reviewWorkerCancel
	a.reviewWorkerMu.Unlock()
	if cancel != nil {
		cancel()
		a.reviewWorkerWG.Wait()
	}
}

// claimModerationTask 用跳过锁定行的租约领取任务，过期任务可在重启后恢复。
func (a *App) claimModerationTask(ctx context.Context) (ModerationTask, error) {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return ModerationTask{}, err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT data FROM moderation_tasks WHERE data->>'status'='ai_pending' OR (data->>'status'='ai_processing' AND (data->>'lease_until')::timestamptz<NOW()) ORDER BY data->>'created_at',id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&raw)
	if err != nil {
		return ModerationTask{}, err
	}
	var task ModerationTask
	if err = json.Unmarshal(raw, &task); err != nil {
		return task, err
	}
	task.Status = "ai_processing"
	task.Attempts++
	task.LeaseToken = newID()
	task.LeaseUntil = reviewLeaseExpiry()
	task.UpdatedAt = now()
	if err = putRaw(ctx, tx, "moderation_tasks", task.ID, task); err != nil {
		return task, err
	}
	return task, tx.Commit(ctx)
}

// processModerationOne 处理一个任务；AI未配置或故障进入人工队列，发布内容已成功保存。
func (a *App) processModerationOne(ctx context.Context) (bool, error) {
	// 对已删除公开图的剩余引用持久补偿；覆盖进程在主拒绝事务提交后退出的情况。
	revoked, cleanupErr := list[ModerationTask](ctx, a.DB, "moderation_tasks", `WHERE data->>'kind'='image' AND data->>'status' IN ('ai_pending','ai_processing','manual_pending','allowed') AND EXISTS (SELECT 1 FROM evidence e WHERE e.id=moderation_tasks.data->>'media_id' AND e.data->>'moderation'='rejected') ORDER BY data->>'created_at',id LIMIT 1`)
	if cleanupErr != nil {
		return false, cleanupErr
	}
	if len(revoked) > 0 {
		err := a.finishModeration(ctx, revoked[0], "reject", "该图片已被审核拒绝", "source", "")
		if ae, ok := err.(*apiError); ok && ae.Status == 409 {
			return true, nil
		}
		return true, err
	}
	task, err := a.claimModerationTask(ctx)
	if missing(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if current, checkErr := a.isReviewTaskCurrent(ctx, task); checkErr != nil {
		return true, a.manualModeration(ctx, task, "版本信息暂不可读取，等待人工复核")
	} else if !current {
		return true, a.finishModeration(ctx, task, "superseded", "内容已更新", "system", task.LeaseToken)
	}
	images := []moderationImage{}
	if task.Kind == "image" {
		media, e := get[Evidence](ctx, a.DB, "evidence", task.MediaID, false)
		if e != nil {
			return true, a.manualModeration(ctx, task, "原图暂不可读取")
		}
		if contains([]string{"ai", "manual", "trusted_platform"}, media.Moderation) {
			return true, a.finishModeration(ctx, task, "allow", "复用同一图片已完成的审核结果", "source", task.LeaseToken)
		}
		if task.TargetType != "evidence" && media.Moderation != "rejected" {
			return true, a.manualModeration(ctx, task, "等待原始图片审核结果；无需重复提交")
		}
		if media.Moderation == "rejected" {
			return true, a.finishModeration(ctx, task, "reject", "该图片已被审核拒绝", "source", task.LeaseToken)
		}
		if media.SHA256 != task.SourceSHA || len(media.Data) == 0 {
			return true, a.manualModeration(ctx, task, "原图摘要不符或不存在")
		}
		images = append(images, moderationImage{MIME: media.MIME, Data: media.Data})
	}
	cfg, e := a.readAISettings(ctx, a.DB)
	if e != nil || cfg.APIKey == "" || validateAISettings(cfg) != nil {
		return true, a.manualModeration(ctx, task, "未配置可用AI，等待管理员人工审核")
	}
	payload, _ := json.Marshal(map[string]string{"context": task.FieldLabel, "text": task.Preview})
	result, e := a.callAI(ctx, cfg, string(payload), images)
	if e != nil {
		return true, a.manualModeration(ctx, task, "AI暂不可用，已转人工审核")
	}
	return true, a.finishModeration(ctx, task, result.Decision, result.Reason, "ai", task.LeaseToken)
}

// isReviewTaskCurrent 在昂贵外部请求前检查任务是否仍指向同一字段版本。
func (a *App) isReviewTaskCurrent(ctx context.Context, task ModerationTask) (bool, error) {
	v, e := get[moderationTarget](ctx, a.DB, "moderation_targets", moderationTargetID(task.TargetType, task.TargetID, task.FieldCode), false)
	if missing(e) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	return v.TaskID == task.ID && v.Version == task.ContentVersion && v.ValueHash == task.ValueHash, nil
}

// manualModeration 只转换仍持有相同租约的AI任务，不覆盖管理员先完成的决定。
func (a *App) manualModeration(ctx context.Context, task ModerationTask, reason string) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	current, err := get[ModerationTask](ctx, tx, "moderation_tasks", task.ID, true)
	if err != nil {
		return err
	}
	if current.Status != "ai_processing" || current.LeaseToken != task.LeaseToken {
		return nil
	}
	current.Status = "manual_pending"
	current.LastError = reason
	current.LeaseToken = ""
	current.LeaseUntil = ""
	current.UpdatedAt = now()
	if err = putRaw(ctx, tx, "moderation_tasks", current.ID, current); err != nil {
		return err
	}
	target, e := get[moderationTarget](ctx, tx, "moderation_targets", moderationTargetID(current.TargetType, current.TargetID, current.FieldCode), true)
	if e != nil && !missing(e) {
		return e
	}
	if e == nil && target.TaskID == current.ID && target.Version == current.ContentVersion {
		target.Status = "manual_pending"
		if e = putRaw(ctx, tx, "moderation_targets", target.ID, target); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// finishModeration 原子比较版本并应用拒绝、撤销公开图及发送一次通知；交易原件从不被覆盖。
func (a *App) finishModeration(ctx context.Context, task ModerationTask, decision, reason, source, lease string) error {
	ctx = withoutReview(ctx)
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	current, err := get[ModerationTask](ctx, tx, "moderation_tasks", task.ID, true)
	if err != nil {
		return err
	}
	if lease != "" {
		if current.Status != "ai_processing" || current.LeaseToken != lease {
			return nil
		}
	} else if !contains([]string{"ai_pending", "ai_processing", "manual_pending"}, current.Status) && !(source == "source" && decision == "reject" && current.Status == "allowed") {
		return fail(409, "该内容已经处理或更新，请刷新审核列表")
	}
	// 与业务写入保持同样的记录行锁到字段版本锁顺序。
	var raw []byte
	err = tx.QueryRow(ctx, "SELECT data FROM "+validTable(current.TargetType)+" WHERE id=$1 FOR UPDATE", current.TargetID).Scan(&raw)
	if err != nil && !missing(err) {
		return err
	}
	target, e := get[moderationTarget](ctx, tx, "moderation_targets", moderationTargetID(current.TargetType, current.TargetID, current.FieldCode), true)
	if e != nil && !missing(e) {
		return e
	}
	if missing(err) || missing(e) || target.TaskID != current.ID || target.Version != current.ContentVersion || target.ValueHash != current.ValueHash || decision == "superseded" {
		current.Status = "superseded"
		current.Reason = "内容已更新，旧审核结果不再应用"
	} else {
		data, e := decodeReviewObject(raw)
		if e != nil {
			return e
		}
		valid := false
		for _, f := range registeredReviewFields(current.TargetType, current.TargetID, data) {
			if f.Code == current.FieldCode && reviewHash(f.Value) == current.ValueHash {
				valid = true
				break
			}
		}
		if current.TargetType == "evidence" && decision == "reject" && reviewString(data, "moderation") == "rejected" && reviewHash(reviewString(data, "sha256")) == current.ValueHash {
			valid = true
		}
		if !valid {
			current.Status = "superseded"
			current.Reason = "内容已更新，旧审核结果不再应用"
		} else {
			current.Reason = reason
			current.DecisionSource = source
			current.ReviewedBy = source
			if source == "manual" {
				if state, _ := ctx.Value(reviewContextKey{}).(*reviewRequest); state != nil {
					current.ReviewedBy = state.ActorID
				}
			}
			switch decision {
			case "allow":
				if current.Kind == "image" {
					media, e := get[Evidence](ctx, tx, "evidence", current.MediaID, true)
					if e != nil {
						return e
					}
					if media.Moderation == "rejected" || len(media.Data) == 0 {
						return fail(409, "图片已被撤销，请用户重新上传")
					}
					if media.SHA256 != current.SourceSHA {
						return fail(409, "图片内容版本已经变化")
					}
					if _, err = tx.Exec(ctx, `UPDATE evidence SET data=jsonb_set(data,'{moderation}',to_jsonb($2::text),true) WHERE id=$1`, media.ID, reviewImageApprovalStamp(media.Moderation, source)); err != nil {
						return err
					}
				}
				current.Status = "allowed"
			case "reject":
				current.Status = "rejected"
				if current.Kind == "image" {
					if err = revokeReviewImage(ctx, tx, current.MediaID); err != nil {
						return err
					}
				}
				if !current.Protected && current.TargetType != "evidence" {
					if !setReviewPath(data, current.Path, current.Replacement) {
						return errors.New("已登记审核字段路径失效")
					}
					if err = putRaw(ctx, tx, current.TargetType, current.TargetID, data); err != nil {
						return err
					}
				}
				if err = sendReviewTaskNotice(ctx, tx, current); err != nil {
					return err
				}
			default:
				return fail(400, "审核决定无效")
			}
			target.Status = current.Status
			if err = putRaw(ctx, tx, "moderation_targets", target.ID, target); err != nil {
				return err
			}
		}
	}
	current.LeaseToken = ""
	current.LeaseUntil = ""
	current.LastError = ""
	current.UpdatedAt = now()
	if err = putRaw(ctx, tx, "moderation_tasks", current.ID, current); err != nil {
		return err
	}
	if err = audit(ctx, tx, current.ReviewedBy, "moderation."+current.Status, current.ID, current.FieldLabel); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if current.Status == "superseded" && lease == "" {
		return fail(409, "内容已更新，旧审核任务不能修改新内容")
	}
	if current.Status == "allowed" && current.MediaID != "" && current.TargetType == "evidence" {
		return a.propagateAcceptedImage(ctx, current.MediaID, current.ID)
	}
	if current.Status == "rejected" && current.MediaID != "" {
		return a.propagateRevokedImage(ctx, current.MediaID, current.ID)
	}
	return nil
}

// revokeReviewImage 删除公开违规字节并保留摘要墓碑；订单证据只隔离，原件仍供平台争议审查。
func revokeReviewImage(ctx context.Context, q querier, id string) error {
	image, err := get[Evidence](ctx, q, "evidence", id, true)
	if err != nil {
		return err
	}
	if image.Public {
		_, err = q.Exec(ctx, `UPDATE evidence SET data=jsonb_set(jsonb_set(data,'{moderation}','"rejected"'::jsonb,true),'{data}','null'::jsonb,true) WHERE id=$1`, id)
	} else {
		_, err = q.Exec(ctx, `UPDATE evidence SET data=jsonb_set(data,'{moderation}','"rejected"'::jsonb,true) WHERE id=$1`, id)
	}
	return err
}

// propagateRevokedImage 撤销同一图片当前有效的其他引用，旧版本引用不会改写新图片。
func (a *App) propagateRevokedImage(ctx context.Context, mediaID, sourceID string) error {
	tasks, err := list[ModerationTask](ctx, a.DB, "moderation_tasks", "WHERE data->>'media_id'=$1 AND id<>$2 AND data->>'status' IN ('ai_pending','ai_processing','manual_pending','allowed')", mediaID, sourceID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err = a.finishModeration(ctx, task, "reject", "该图片已被审核拒绝", "source", ""); err != nil {
			var ae *apiError
			if !errors.As(err, &ae) || ae.Status != 409 {
				return err
			}
		}
	}
	return nil
}

// sendReviewTaskNotice 以不可复用任务ID保证通知幂等，通知已读删除后也不会被重复生成。
func sendReviewTaskNotice(ctx context.Context, q querier, task ModerationTask) error {
	id := "review_task_" + task.ID
	path, label := reviewNoticeAction(task)
	v := InboxMessage{ID: id, UserID: task.OwnerID, Title: "图文审核未通过", TargetType: task.TargetType, TargetID: task.TargetID, FieldLabel: task.FieldLabel, ActionPath: path, ActionLabel: label, Body: "位置：" + task.FieldLabel + "\n内容标识：" + task.TargetID + "\n原因：" + task.Reason + "\n相关内容已隐藏，请修改后重新提交；如有疑问请联系平台客服。", CreatedAt: now()}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, `WITH claimed AS (INSERT INTO inbox_receipts(id,data) VALUES($1,jsonb_build_object('task_id',$3::text)) ON CONFLICT(id) DO NOTHING RETURNING id) INSERT INTO inbox(id,data) SELECT id,$2::jsonb FROM claimed`, id, raw, task.ID)
	return err
}

// reviewNoticeAction 只生成硬编码站内修改或查看路由，普通用户不会收到管理员审核URL。
func reviewNoticeAction(task ModerationTask) (string, string) {
	id := url.QueryEscape(task.TargetID)
	switch task.TargetType {
	case "users":
		return "/pages/account/profile", "修改个人资料"
	case "services":
		return "/pages/services/publish?id=" + id, "修改服务内容"
	case "clubs":
		return "/pages/community/clubs/edit?id=" + id, "修改俱乐部资料"
	case "members":
		club := strings.SplitN(task.TargetID, ":", 2)[0]
		if task.FieldCode == "application_note" {
			return "/pages/community/clubs/detail?id=" + url.QueryEscape(club), "查看并修改入会申请"
		}
		return "/pages/community/clubs/console?id=" + url.QueryEscape(club), "查看俱乐部申请"
	case "orders":
		return "/pages/orders/detail?id=" + id, "查看订单并联系平台"
	case "messages", "reviews", "order_adjustments":
		if task.OrderID != "" {
			return "/pages/orders/detail?id=" + url.QueryEscape(task.OrderID), "查看订单并联系平台"
		}
	case "settings":
		if task.TargetID == publicContentID {
			return "/pages/manage/content/index", "修改公共信息"
		}
		if strings.HasPrefix(task.FieldCode, "support_") {
			return "/pages/manage/support", "修改客服信息"
		}
		return "/pages/manage/platform", "修改平台公开信息"
	case "games":
		return "/pages/manage/games", "修改游戏资料"
	case "articles":
		return "/pages/manage/articles/edit?id=" + id, "修改文章内容"
	case "inbox":
		return "/pages/manage/messages", "查看平台通知"
	case "evidence":
		if task.Purpose == "promotion_banner" || task.Purpose == "announcement_image" {
			return "/pages/manage/content/index", "重新上传公共图片"
		}
		if task.Purpose == "article_cover" || task.Purpose == "article_image" {
			return "/pages/manage/articles/index", "检查文章图片"
		}
		if task.Purpose == "support_qr" {
			return "/pages/manage/support", "重新上传客服二维码"
		}
		if strings.HasPrefix(task.Purpose, "club_") {
			if task.ClubID != "" {
				return "/pages/community/clubs/edit?id=" + url.QueryEscape(task.ClubID), "重新上传俱乐部图片"
			}
			return "/pages/community/clubs/index", "重新上传俱乐部图片"
		}
		if task.Purpose == "" && task.OrderID == "" {
			return "/pages/manage/games", "重新上传游戏素材"
		}
		if task.OrderID != "" {
			return "/pages/orders/detail?id=" + url.QueryEscape(task.OrderID), "查看订单并重新提交"
		}
	}
	return "/pages/account/profile", "检查资料并重新上传"
}

// reviewImageApprovalStamp 复用已有图像决定时保留AI或人工来源，不把内部级联标签存成新的审核方式。
func reviewImageApprovalStamp(existing, source string) string {
	if source == "source" && contains([]string{"ai", "manual", "trusted_platform"}, existing) {
		return existing
	}
	return source
}

// propagateAcceptedImage 让同一原图的有效等待引用复用决定，避免重复发送图片及无AI时重复人工操作。
func (a *App) propagateAcceptedImage(ctx context.Context, mediaID, sourceID string) error {
	tasks, err := list[ModerationTask](ctx, a.DB, "moderation_tasks", "WHERE data->>'media_id'=$1 AND id<>$2 AND data->>'status' IN ('ai_pending','ai_processing','manual_pending')", mediaID, sourceID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err = a.finishModeration(ctx, task, "allow", "复用同一图片已完成的审核结果", "source", ""); err != nil {
			var ae *apiError
			if !errors.As(err, &ae) || ae.Status != 409 {
				return err
			}
		}
	}
	return nil
}
