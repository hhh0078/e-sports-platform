// Package app 将图文保存与持久化审核任务绑定在同一事务，AI故障只改变队列状态。
package app

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"reflect"
	"strings"
	"time"
)

type reviewContextKey struct{}
type reviewSuppressionKey struct{}

// reviewRequest 收集当前成功写入产生的任务，不保存支付、登录秘密。
type reviewRequest struct {
	App            *App
	ActorID        string
	Mode           string
	RawAdminReview bool
	TaskIDs        []string
}

// ModerationTask 是管理员可处理的字段级持久任务，内部路径和摘要不向普通用户返回。
type ModerationTask struct {
	ID             string   `json:"id"`
	TargetType     string   `json:"target_type"`
	TargetID       string   `json:"target_id"`
	OrderID        string   `json:"order_id,omitempty"`
	ClubID         string   `json:"club_id,omitempty"`
	Purpose        string   `json:"purpose,omitempty"`
	FieldCode      string   `json:"field_code"`
	FieldLabel     string   `json:"field_label"`
	Kind           string   `json:"kind"`
	Status         string   `json:"status"`
	ActorID        string   `json:"actor_id"`
	OwnerID        string   `json:"owner_id"`
	ContentVersion int64    `json:"content_version"`
	ValueHash      string   `json:"value_hash"`
	Preview        string   `json:"preview"`
	MediaID        string   `json:"media_id,omitempty"`
	SourceSHA      string   `json:"source_sha,omitempty"`
	Path           []string `json:"path"`
	Protected      bool     `json:"protected"`
	Replacement    string   `json:"replacement"`
	Attempts       int      `json:"attempts"`
	LeaseToken     string   `json:"lease_token,omitempty"`
	LeaseUntil     string   `json:"lease_until,omitempty"`
	Reason         string   `json:"reason"`
	LastError      string   `json:"last_error"`
	DecisionSource string   `json:"decision_source"`
	ReviewedBy     string   `json:"reviewed_by,omitempty"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// moderationTarget 保存字段单调版本，确保A到B再到A时旧任务仍不能覆盖新提交。
type moderationTarget struct {
	ID          string   `json:"id"`
	TargetType  string   `json:"target_type"`
	TargetID    string   `json:"target_id"`
	FieldCode   string   `json:"field_code"`
	Version     int64    `json:"version"`
	ValueHash   string   `json:"value_hash"`
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	Path        []string `json:"path"`
	Replacement string   `json:"replacement"`
	Protected   bool     `json:"protected"`
}

// moderationTargetID 把白名单目标与字段编码为固定长度主键。
func moderationTargetID(table, id, field string) string {
	return reviewHash(table + "\n" + id + "\n" + field)
}

// reviewContext 在HTTP业务写入时启用审核，初始化种子、微信回调和后台处理不递归创建任务。
func (a *App) reviewContext(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), reviewContextKey{}, &reviewRequest{App: a, RawAdminReview: strings.HasPrefix(r.URL.Path, "/api/admin/moderation")}))
}

// withoutReview 为审核结果更新禁用再次入队，原业务事务及权限保持不变。
func withoutReview(ctx context.Context) context.Context {
	return context.WithValue(ctx, reviewSuppressionKey{}, true)
}

// reviewWriteState 只返回已认证且未被内部禁用的当前请求审核收集器。
func reviewWriteState(ctx context.Context) *reviewRequest {
	if v, _ := ctx.Value(reviewSuppressionKey{}).(bool); v {
		return nil
	}
	s, _ := ctx.Value(reviewContextKey{}).(*reviewRequest)
	if s == nil || s.ActorID == "" {
		return nil
	}
	return s
}

// moderationInitialStatus 读取可用AI配置，无配置或解密故障均进入人工队列而不影响保存。
func moderationInitialStatus(ctx context.Context, q querier, s *reviewRequest) string {
	if s.Mode != "" {
		return s.Mode
	}
	s.Mode = "manual_pending"
	cfg, err := s.App.readAISettings(ctx, q)
	if err == nil && cfg.APIKey != "" && validateAISettings(cfg) == nil {
		s.Mode = "ai_pending"
	}
	return s.Mode
}

// putWithReview 在原记录行锁中比较登记字段，将数据和新审核任务原子持久化。
func putWithReview(ctx context.Context, q querier, table, id string, value any, s *reviewRequest) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	next, err := decodeReviewObject(raw)
	if err != nil {
		return err
	}
	fields := registeredReviewFields(table, id, next)
	if len(fields) == 0 && !reviewedTable(table) {
		return putRaw(ctx, q, table, id, value)
	}
	var previous []byte
	err = q.QueryRow(ctx, "SELECT data FROM "+validTable(table)+" WHERE id=$1 FOR UPDATE", id).Scan(&previous)
	if err != nil && !missing(err) {
		return err
	}
	old, err := decodeReviewObject(previous)
	if err != nil {
		return err
	}
	oldFields := map[string]reviewField{}
	for _, field := range registeredReviewFields(table, id, old) {
		oldFields[field.Code] = field
	}
	if err = putRaw(ctx, q, table, id, value); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, field := range fields {
		seen[field.Code] = true
		prior, exists := oldFields[field.Code]
		if exists && prior.Value == field.Value && reflect.DeepEqual(prior.Path, field.Path) {
			continue
		}
		if err = enqueueReviewField(ctx, q, s, table, id, next, field); err != nil {
			return err
		}
	}
	// 清空或删除字段也增加版本，禁止迟到结果重新填回旧内容。
	for code := range oldFields {
		if !seen[code] {
			key := moderationTargetID(table, id, code)
			target, e := get[moderationTarget](ctx, q, "moderation_targets", key, true)
			if e != nil && !missing(e) {
				return e
			}
			if e == nil {
				target.Version++
				target.Status = "superseded"
				target.TaskID = ""
				target.ValueHash = ""
				if e = putRaw(ctx, q, "moderation_targets", key, target); e != nil {
					return e
				}
			}
		}
	}
	return nil
}

// reviewedTable 将审核钩子限定于真实图文业务表，资金事实不进入审核模块。
func reviewedTable(table string) bool {
	return contains([]string{"users", "services", "clubs", "members", "orders", "messages", "reviews", "evidence", "order_adjustments", "settings", "games", "inbox"}, table)
}

// enqueueReviewField 创建带当前版本、归属与图片摘要的任务，任务持久失败会回滚本次业务保存。
func enqueueReviewField(ctx context.Context, q querier, s *reviewRequest, table, id string, data map[string]any, field reviewField) error {
	key := moderationTargetID(table, id, field.Code)
	target, err := get[moderationTarget](ctx, q, "moderation_targets", key, true)
	if err != nil && !missing(err) {
		return err
	}
	stamp := now()
	task := ModerationTask{ID: newID(), TargetType: table, TargetID: id, OrderID: reviewString(data, "order_id"), ClubID: reviewString(data, "club_id"), Purpose: reviewString(data, "purpose"), FieldCode: field.Code, FieldLabel: field.Label, Kind: field.Kind, Status: moderationInitialStatus(ctx, q, s), ActorID: s.ActorID, OwnerID: reviewOwner(table, data, s.ActorID), ContentVersion: target.Version + 1, ValueHash: reviewHash(field.Value), Preview: field.Value, MediaID: field.MediaID, Path: field.Path, Protected: field.Protected, Replacement: field.Replacement, CreatedAt: stamp, UpdatedAt: stamp}
	if field.Kind == "image" {
		task.Preview = ""
		media, e := get[Evidence](ctx, q, "evidence", field.MediaID, false)
		if e != nil {
			return e
		}
		// 同一不可变媒体只安排一个实际图像审核，业务字段任务保留各自归属与版本。
		if table != "evidence" && !contains([]string{"ai", "manual", "trusted_platform", "rejected"}, media.Moderation) {
			sourceKey := moderationTargetID("evidence", field.MediaID, "image")
			if _, checkErr := get[moderationTarget](ctx, q, "moderation_targets", sourceKey, false); missing(checkErr) {
				object, e := get[map[string]any](ctx, q, "evidence", field.MediaID, false)
				if e != nil {
					return e
				}
				sourceFields := registeredReviewFields("evidence", field.MediaID, object)
				for _, sourceField := range sourceFields {
					if e = enqueueReviewField(ctx, q, s, "evidence", field.MediaID, object, sourceField); e != nil {
						return e
					}
				}
			} else if checkErr != nil {
				return checkErr
			}
		}
		task.SourceSHA = media.SHA256
		if media.Moderation == "ai" || media.Moderation == "manual" || media.Moderation == "trusted_platform" {
			task.Status = "allowed"
			task.DecisionSource = "inherited"
		}
	}
	target = moderationTarget{ID: key, TargetType: table, TargetID: id, FieldCode: field.Code, Version: task.ContentVersion, ValueHash: task.ValueHash, TaskID: task.ID, Status: task.Status, Path: field.Path, Replacement: field.Replacement, Protected: field.Protected}
	if err = putRaw(ctx, q, "moderation_targets", key, target); err != nil {
		return err
	}
	if err = putRaw(ctx, q, "moderation_tasks", task.ID, task); err != nil {
		return err
	}
	if task.Status != "allowed" {
		s.TaskIDs = append(s.TaskIDs, task.ID)
	}
	return nil
}

// reviewResponseMeta 仅对成功创建审核任务的响应返回轻提示状态，旧任务不重复触发提示。
func reviewResponseMeta(ctx context.Context) map[string]any {
	s := reviewWriteState(ctx)
	if s == nil || len(s.TaskIDs) == 0 {
		return nil
	}
	return map[string]any{"status": s.Mode, "estimated_seconds": 20, "pending_count": len(s.TaskIDs)}
}

// reviewPoolWrite 使原先直接用连接池写入的旧接口也获得数据与审核任务的原子性。
func reviewPoolWrite(ctx context.Context, pool *pgxpool.Pool, table, id string, value any, s *reviewRequest) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = putWithReview(ctx, tx, table, id, value, s); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// reviewLeaseExpiry 为处理进程预留有限租约，服务重启后可被其他实例继续领取。
func reviewLeaseExpiry() string {
	return time.Now().Add(45 * time.Second).UTC().Format(time.RFC3339Nano)
}
