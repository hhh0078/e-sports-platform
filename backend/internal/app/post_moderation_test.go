// Package app 验证后置审核、持久队列、人工/AI竞争、字段版本、原证据与资金隔离。
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// latestReviewTask 读取指定字段最近的单调版本任务，找不到时使测试明确失败。
func latestReviewTask(t *testing.T, api *testAPI, table, id, field string) ModerationTask {
	t.Helper()
	tasks, err := list[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", "WHERE data->>'target_type'=$1 AND data->>'target_id'=$2 AND data->>'field_code'=$3 ORDER BY (data->>'content_version')::bigint DESC LIMIT 1", table, id, field)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("缺少字段审核任务 %s/%s/%s：%v", table, id, field, err)
	}
	return tasks[0]
}

// TestReviewRegistryBoundary 验证全部公开/交易内容登记且不把认证、价格、ID和颜色作为待审文字。
func TestReviewRegistryBoundary(t *testing.T) {
	tests := []struct {
		table, id, raw string
		fields         []string
	}{{"users", "u", `{"id":"u","nickname":"昵称","bio":"简介","avatar":"/api/media/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","profile_cover":"/api/media/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","phone_masked":"138****0000"}`, []string{"nickname", "bio", "avatar", "profile_cover"}}, {"services", "s", `{"title":"标题","description":"说明","unit":"局","tags":["标签"],"labels":[{"id":"l","text":"文案","color":"#FFFFFF"}],"price_cents":900}`, []string{"title", "description", "unit", "tags.0", "label.l"}}, {"clubs", "c", `{"name":"队伍","description":"简介","assessment":"考核","join_note":"备注","contact_phone":"13800138000","contact_wechat":"club","detail_blocks":[{"type":"text","text":"详情"},{"type":"image","image_url":"/api/media/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`, []string{"name", "description", "assessment", "join_note", "contact_phone", "contact_wechat", "detail.0.text", "detail.1.image_url"}}, {"members", "c:u", `{"application_note":"申请","review_note":"回复"}`, []string{"application_note", "review_note"}}, {"orders", "o", `{"title":"需求","requirements":"约定","service_description":"快照","delivery_note":"交付","refund_reason":"原因","refund_reply":"回复","dispute_reason":"争议","amount_cents":900}`, []string{"title", "requirements", "service_description", "delivery_note", "refund_reason", "refund_reply", "dispute_reason"}}, {"messages", "m", `{"content":"正文","image_url":"/api/evidence/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, []string{"content", "image_url"}}, {"reviews", "r", `{"content":"评价"}`, []string{"content"}}, {"order_adjustments", "a", `{"reason":"协商"}`, []string{"reason"}}}
	for _, test := range tests {
		t.Run(test.table, func(t *testing.T) {
			v, _ := decodeReviewObject([]byte(test.raw))
			got := map[string]bool{}
			for _, f := range registeredReviewFields(test.table, test.id, v) {
				got[f.Code] = true
			}
			if len(got) != len(test.fields) {
				t.Fatalf("登记数量错误%v", got)
			}
			for _, field := range test.fields {
				if !got[field] {
					t.Fatalf("字段未登记%s", field)
				}
			}
		})
	}
}

// TestIntegrationPostReviewManualAndNotifications 验证无AI正常保存、越权隔离、人工拒绝和通知永久幂等。
func TestIntegrationPostReviewManualAndNotifications(t *testing.T) {
	api := newTestAPI(t)
	api.App.Config.DevMode = false
	code, raw, err := api.request("worker", "PATCH", "/me/profile", map[string]string{"bio": "人工待审正常保存的游戏介绍"})
	if err != nil || code != 200 || !strings.Contains(string(raw), "manual_pending") {
		t.Fatalf("保存被审核拦住%d %s", code, raw)
	}
	task := latestReviewTask(t, api, "users", "worker", "bio")
	if task.Status != "manual_pending" {
		t.Fatal("无AI没有待人工")
	}
	for _, actor := range []string{"", "player", "worker", "clubowner", "outsider"} {
		want := 403
		if actor == "" {
			want = 401
		}
		api.call(actor, "GET", "/admin/moderation", nil, want, nil)
		api.call(actor, "GET", "/admin/moderation/"+task.ID, nil, want, nil)
		api.call(actor, "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "不合规"}, want, nil)
	}
	var decided map[string]any
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "包含违规内容"}, 200, &decided)
	if decided["status"] != "rejected" {
		t.Fatal("决定没有返回完整任务")
	}
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "allow"}, 409, nil)
	saved, _ := get[User](t.Context(), api.App.DB, "users", "worker", false)
	if saved.Bio != rejectedContentText {
		t.Fatal("公开字段没有替换")
	}
	var inbox struct {
		Items []InboxMessage `json:"items"`
		Total int            `json:"total"`
	}
	api.call("worker", "GET", "/messages", nil, 200, &inbox)
	if inbox.Total != 1 || !strings.Contains(inbox.Items[0].Body, "包含违规内容") {
		t.Fatal("缺少字段拒绝通知")
	}
	api.call("worker", "DELETE", "/messages/"+inbox.Items[0].ID, nil, 200, nil)
	if err = sendReviewTaskNotice(t.Context(), api.App.DB, task); err != nil {
		t.Fatal(err)
	}
	api.call("worker", "GET", "/messages", nil, 200, &inbox)
	if inbox.Total != 0 {
		t.Fatal("已读通知被重复生成")
	}
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "修正后的正常游戏介绍"}, 200, nil)
	newTask := latestReviewTask(t, api, "users", "worker", "bio")
	if newTask.ContentVersion != 2 {
		t.Fatal("修改未增加字段版本")
	}
	api.call("admin", "POST", "/admin/moderation/"+newTask.ID+"/retry", map[string]any{}, 200, &decided)
	if decided["status"] != "manual_pending" {
		t.Fatal("无AI重试不应伪装审核通过")
	}
}

// TestIntegrationPostReviewABATransactionAndDeletion 验证A到B到A、清空字段、业务事务回滚均不会被旧任务覆盖。
func TestIntegrationPostReviewABATransactionAndDeletion(t *testing.T) {
	api := newTestAPI(t)
	for _, bio := range []string{"第一版内容A", "第二版内容B", "第一版内容A"} {
		api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": bio}, 200, nil)
	}
	tasks, err := list[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", "WHERE data->>'target_id'='worker' AND data->>'field_code'='bio' ORDER BY (data->>'content_version')::bigint")
	if err != nil || len(tasks) != 3 {
		t.Fatal("缺少ABA版本")
	}
	if tasks[0].ValueHash != tasks[2].ValueHash || tasks[0].ContentVersion == tasks[2].ContentVersion {
		t.Fatal("内容hash代替了版本")
	}
	api.call("admin", "POST", "/admin/moderation/"+tasks[0].ID+"/decision", map[string]string{"decision": "reject", "reason": "旧版本"}, 409, nil)
	saved, _ := get[User](t.Context(), api.App.DB, "users", "worker", false)
	if saved.Bio != "第一版内容A" {
		t.Fatal("迟到旧任务覆盖新内容")
	}
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": ""}, 200, nil)
	api.call("admin", "POST", "/admin/moderation/"+tasks[2].ID+"/decision", map[string]string{"decision": "reject", "reason": "已清空旧内容"}, 409, nil)
	saved, _ = get[User](t.Context(), api.App.DB, "users", "worker", false)
	if saved.Bio != "" {
		t.Fatal("旧审核重新填回已清空内容")
	}
	req := api.App.reviewContext(httptest.NewRequest("PATCH", "/api/me/profile", nil))
	state := req.Context().Value(reviewContextKey{}).(*reviewRequest)
	state.ActorID = "worker"
	tx, err := api.App.DB.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	saved.Bio = "回滚事务中的未发布介绍"
	if err = put(req.Context(), tx, "users", "worker", saved); err != nil {
		t.Fatal(err)
	}
	ids := append([]string{}, state.TaskIDs...)
	tx.Rollback(t.Context())
	saved, _ = get[User](t.Context(), api.App.DB, "users", "worker", false)
	if saved.Bio != "" {
		t.Fatal("回滚后仍有业务数据")
	}
	for _, id := range ids {
		if _, err = get[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", id, false); !missing(err) {
			t.Fatal("业务回滚留下孤立审核任务")
		}
	}
}

// TestIntegrationPostReviewAIManualRaceAndLease 验证人工先决定、AI迟到、过期租约恢复与多次处理幂等。
func TestIntegrationPostReviewAIManualRaceAndLease(t *testing.T) {
	api := newTestAPI(t)
	entered, release := make(chan struct{}), make(chan struct{})
	installMockAI(t, api, func(*http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return aiMockResponse(`{"decision":"reject","reason":"迟到AI拒绝"}`), nil
	})
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "人工将确认通过的资料"}, 200, nil)
	task := latestReviewTask(t, api, "users", "worker", "bio")
	done := make(chan error, 1)
	go func() { _, err := api.App.processModerationOne(context.Background()); done <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("AI未领取任务")
	}
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "allow", "reason": "人工核验通过"}, 200, nil)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	current, _ := get[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", task.ID, false)
	if current.Status != "allowed" || current.DecisionSource != "manual" {
		t.Fatal("AI迟到覆盖人工决定")
	}
	api.App.AIHTTPClient = &http.Client{Transport: paymentTransport(func(*http.Request) (*http.Response, error) {
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})}
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "租约过期后继续审查的资料"}, 200, nil)
	leased, err := api.App.claimModerationTask(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	leased.LeaseUntil = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if err = putRaw(t.Context(), api.App.DB, "moderation_tasks", leased.ID, leased); err != nil {
		t.Fatal(err)
	}
	drainReviewAI(t, api)
	current, _ = get[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", leased.ID, false)
	if current.Status != "allowed" || current.Attempts != 2 {
		t.Fatal("重启租约未恢复或重复处理")
	}
}

// TestIntegrationPostReviewPrivateEvidenceAndFunding 验证原聊天与调价理由保留、普通显示屏蔽、资金不被审核改变。
func TestIntegrationPostReviewPrivateEvidenceAndFunding(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	api.action("worker", o.ID, "accept", "", 200)
	api.action("worker", o.ID, "start", "", 200)
	var message Message
	api.call("player", "POST", "/orders/"+o.ID+"/messages", map[string]string{"content": "需隔离保留的聊天原始内容"}, 200, &message)
	task := latestReviewTask(t, api, "messages", message.ID, "content")
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "不适合展示"}, 200, nil)
	original, _ := get[Message](t.Context(), api.App.DB, "messages", message.ID, false)
	if original.Content != message.Content {
		t.Fatal("聊天原证据被物理覆盖")
	}
	var messages []Message
	api.call("player", "GET", "/orders/"+o.ID+"/messages", nil, 200, &messages)
	for _, v := range messages {
		if v.ID == message.ID && v.Content != rejectedContentText {
			t.Fatal("普通聊天仍显示违规原文")
		}
	}
	var adjustment OrderAdjustment
	body := map[string]any{"kind": "increase", "amount_cents": 100, "reason": "需隔离的调价原始理由", "request_id": "moderation-adjustment-1"}
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments", body, 200, &adjustment)
	changeTask := latestReviewTask(t, api, "order_adjustments", adjustment.ID, "reason")
	api.call("admin", "POST", "/admin/moderation/"+changeTask.ID+"/decision", map[string]string{"decision": "reject", "reason": "理由违规"}, 200, nil)
	raw, _ := get[OrderAdjustment](t.Context(), api.App.DB, "order_adjustments", adjustment.ID, false)
	if raw.Reason != body["reason"] {
		t.Fatal("资金幂等原事实被修改")
	}
	api.call("worker", "POST", "/orders/"+o.ID+"/adjustments", body, 200, nil)
	saved, _ := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if saved.AmountCents != o.AmountCents || saved.PaymentStatus != "paid" {
		t.Fatal("图文审核改变资金")
	}
	evidence := api.uploadTestImage("player", o.ID)
	imageTask := latestReviewTask(t, api, "evidence", reviewMediaID(evidence), "image")
	api.call("admin", "POST", "/admin/moderation/"+imageTask.ID+"/decision", map[string]string{"decision": "reject", "reason": "图片需隔离"}, 200, nil)
	stored, _ := get[Evidence](t.Context(), api.App.DB, "evidence", reviewMediaID(evidence), false)
	if len(stored.Data) == 0 {
		t.Fatal("交易图片原件丢失")
	}
	req, _ := http.NewRequest("GET", api.Server.URL+"/api/admin/moderation/"+imageTask.ID+"/image", nil)
	req.Header.Set("Authorization", "Bearer "+api.Tokens["admin"])
	res, err := api.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Equal(actual, stored.Data) || res.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("管理员无法受限查看原证据")
	}
	api.call("player", "GET", "/admin/moderation/"+imageTask.ID+"/image", nil, 403, nil)
}

// TestIntegrationPostReviewClubValidationAndPagination 验证字段级长度错误、服务先发布与后台游标数量。
func TestIntegrationPostReviewClubValidationAndPagination(t *testing.T) {
	api := newTestAPI(t)
	for _, entry := range []struct{ name, description, expected string }{{"短", "合法的俱乐部详细介绍文字", "名称需为2至30字"}, {"正常俱乐部", "太短", "介绍需为10至1000字"}} {
		code, raw, err := api.request("player", "POST", "/clubs", map[string]any{"name": entry.name, "description": entry.description, "game_ids": []string{"delta"}})
		if err != nil || code != 400 || !strings.Contains(string(raw), entry.expected) {
			t.Fatalf("长度提示不明确%d %s", code, raw)
		}
	}
	var service Service
	api.call("worker", "POST", "/services", map[string]any{"game_id": "delta", "title": "后置审核正常服务", "description": "提供正常游戏陪练与任务协作内容", "category": "任务协作", "price_cents": 1900, "unit": "局", "cover_url": "/static/covers/official/delta-1.jpg", "labels": []CoverLabel{{Text: "游戏文案", Color: "#FFFFFF", Background: "transparent", FontSize: 16}}, "tags": []string{"耐心"}}, 200, &service)
	if service.Status != "active" {
		t.Fatal("服务发布仍被审核状态阻塞")
	}
	api.call("", "GET", "/services/"+service.ID, nil, 200, nil)
	var first struct {
		Items  []ModerationTask `json:"items"`
		Next   string           `json:"next_cursor"`
		Counts map[string]int64 `json:"counts"`
	}
	api.call("admin", "GET", "/admin/moderation?limit=2", nil, 200, &first)
	if len(first.Items) != 2 || first.Next == "" || first.Counts["manual_pending"] < 5 {
		t.Fatal("审核分页或汇总错误")
	}
	var second struct {
		Items []ModerationTask `json:"items"`
	}
	api.call("admin", "GET", "/admin/moderation?limit=2&cursor="+first.Next, nil, 200, &second)
	for _, left := range first.Items {
		for _, right := range second.Items {
			if left.ID == right.ID {
				t.Fatal("稳定游标重复记录")
			}
		}
	}
	api.call("admin", "GET", "/admin/moderation?status=invalid", nil, 400, nil)
}

// TestReviewResponseMetadataBoundary 验证提示仅含状态、数量及估计时间，不包含用户正文或身份秘密。
func TestReviewResponseMetadataBoundary(t *testing.T) {
	state := &reviewRequest{ActorID: "u", Mode: "manual_pending", TaskIDs: []string{"a", "b"}}
	ctx := context.WithValue(context.Background(), reviewContextKey{}, state)
	raw, _ := json.Marshal(reviewResponseMeta(ctx))
	if !strings.Contains(string(raw), `"pending_count":2`) || strings.Contains(string(raw), "TaskIDs") {
		t.Fatal("审核响应元数据错误")
	}
}

// TestIntegrationPostReviewNewAllowedValueNotMasked 验证A被拒后改B再提交A，通过新版本后HTTP显示不受旧拒绝污染。
func TestIntegrationPostReviewNewAllowedValueNotMasked(t *testing.T) {
	api := newTestAPI(t)
	api.call("worker", "PATCH", "/me/profile", map[string]string{"nickname": "曾被误判的昵称"}, 200, nil)
	first := latestReviewTask(t, api, "users", "worker", "nickname")
	api.call("admin", "POST", "/admin/moderation/"+first.ID+"/decision", map[string]string{"decision": "reject", "reason": "首次误判"}, 200, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]string{"nickname": "第二个昵称"}, 200, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]string{"nickname": "曾被误判的昵称"}, 200, nil)
	current := latestReviewTask(t, api, "users", "worker", "nickname")
	api.call("admin", "POST", "/admin/moderation/"+current.ID+"/decision", map[string]string{"decision": "allow", "reason": "人工确认正常"}, 200, nil)
	var user User
	api.call("worker", "GET", "/me", nil, 200, &user)
	if user.Nickname != "曾被误判的昵称" {
		t.Fatal("旧拒绝仍屏蔽当前新允许的昵称")
	}
	order := api.newOrder()
	var returned Order
	api.call("player", "GET", "/orders/"+order.ID, nil, 200, &returned)
	if returned.ProviderName != "曾被误判的昵称" {
		t.Fatal("旧拒绝污染新允许后的交易快照")
	}
}

// TestIntegrationPostReviewAllWriteEntrypoints 验证旧资料、服务、俱乐部、申请、订单、评价及管理员公开内容全部写入持久队列。
func TestIntegrationPostReviewAllWriteEntrypoints(t *testing.T) {
	api := newTestAPI(t)
	api.call("worker", "PATCH", "/me", map[string]string{"nickname": "新的游戏昵称"}, 200, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "新的公开游戏个人简介"}, 200, nil)
	var service Service
	api.call("worker", "POST", "/services", map[string]any{"game_id": "delta", "title": "正常游戏服务标题", "description": "提供正常游戏陪练与任务协作服务说明", "category": "任务协作", "price_cents": 1900, "unit": "局", "cover_url": "/static/covers/official/delta-1.jpg", "labels": []CoverLabel{{Text: "独立文案", Color: "#FFFFFF", Background: "transparent", FontSize: 16}}, "tags": []string{"耐心"}}, 200, &service)
	var club Club
	api.call("player", "POST", "/clubs", map[string]any{"name": "新的游戏俱乐部", "description": "俱乐部有明确的正常游戏服务介绍", "game_ids": []string{"delta"}, "assessment": "常规考核", "join_note": "可换行加入备注\n第二行", "detail_blocks": []CommunityBlock{{Type: "text", Text: "正常俱乐部图文介绍"}}}, 200, &club)
	api.call("outsider", "POST", "/clubs/shanhai/join", map[string]string{"application_note": "提交真实的入会申请"}, 200, nil)
	api.call("clubowner", "POST", "/clubs/shanhai/members/worker", map[string]string{"status": "active", "review_note": "成员考核经过核验"}, 200, nil)
	var request Order
	api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "需要正常游戏队友", "requirements": "需要一起完成正常游戏任务", "amount_cents": 1000}, 200, &request)
	order := api.newOrder()
	api.action("worker", order.ID, "accept", "", 200)
	api.action("worker", order.ID, "start", "", 200)
	api.action("worker", order.ID, "deliver", "已完成全部约定游戏任务", 200)
	api.action("player", order.ID, "confirm", "", 200)
	var review Review
	api.call("player", "POST", "/orders/"+order.ID+"/review", map[string]any{"rating": 5, "content": "这是正常订单服务评价"}, 200, &review)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"announcement": "正常的平台公告内容", "support_phone": "17607080935", "support_wechat": "service_contact"}, 200, nil)
	api.call("admin", "POST", "/admin/messages", map[string]string{"recipient_id": "player", "title": "新的平台消息", "body": "平台发布的正常通知正文"}, 200, nil)
	for _, v := range []struct{ table, id, field string }{{"users", "worker", "nickname"}, {"users", "worker", "bio"}, {"services", service.ID, "title"}, {"services", service.ID, "description"}, {"services", service.ID, "unit"}, {"services", service.ID, "tags.0"}, {"services", service.ID, "label." + service.Labels[0].ID}, {"clubs", club.ID, "name"}, {"clubs", club.ID, "description"}, {"clubs", club.ID, "assessment"}, {"clubs", club.ID, "join_note"}, {"clubs", club.ID, "detail.0.text"}, {"members", "shanhai:outsider", "application_note"}, {"members", "shanhai:worker", "review_note"}, {"orders", request.ID, "title"}, {"orders", request.ID, "requirements"}, {"orders", request.ID, "service_description"}, {"orders", order.ID, "delivery_note"}, {"reviews", review.ID, "content"}, {"settings", "platform", "announcement"}, {"settings", "platform", "support_phone"}, {"settings", "platform", "support_wechat"}} {
		task := latestReviewTask(t, api, v.table, v.id, v.field)
		if task.Status != "manual_pending" {
			t.Fatalf("缺AI没有进入人工%s/%s", v.table, v.field)
		}
	}
	notices, err := list[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", "WHERE data->>'target_type'='inbox'")
	if err != nil || len(notices) != 2 {
		t.Fatal("管理员公开通知绕过图文审核")
	}
}

// TestIntegrationPostReviewAnonymousRequestShare 验证需求标题拒绝后，匿名详情与分享元数据均不泄漏不可变原文。
func TestIntegrationPostReviewAnonymousRequestShare(t *testing.T) {
	api := newTestAPI(t)
	var order Order
	api.call("player", "POST", "/requests", map[string]any{"game_id": "delta", "title": "需要隔离的需求标题", "requirements": "这是正常且完整的需求说明内容", "amount_cents": 1000}, 200, &order)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{}, 200, nil)
	var share ShareMetadata
	api.call("player", "POST", "/shares", map[string]string{"kind": "request", "target_id": order.ID}, 200, &share)
	task := latestReviewTask(t, api, "orders", order.ID, "title")
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "标题不适合公开"}, 200, nil)
	for _, path := range []string{"/jobs/" + order.ID, "/shares/" + share.ID} {
		code, raw, err := api.request("", "GET", path, nil)
		if err != nil || code != 200 || strings.Contains(string(raw), "需要隔离的需求标题") || !strings.Contains(string(raw), rejectedContentText) {
			t.Fatalf("匿名公开入口泄漏原标题%s %d %s", path, code, raw)
		}
	}
	original, _ := get[Order](t.Context(), api.App.DB, "orders", order.ID, false)
	if original.Title != "需要隔离的需求标题" {
		t.Fatal("公开投影修改了原订单")
	}
}

// TestIntegrationPostReviewRevokedApprovedReference 验证上传源图被拒时，已通过的当前引用也同步撤销且可在进程恢复后补偿。
func TestIntegrationPostReviewRevokedApprovedReference(t *testing.T) {
	api := newTestAPI(t)
	url := api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 200)
	api.call("player", "PATCH", "/me/profile", map[string]string{"avatar": url}, 200, nil)
	reference := latestReviewTask(t, api, "users", "player", "avatar")
	api.call("admin", "POST", "/admin/moderation/"+reference.ID+"/decision", map[string]string{"decision": "allow"}, 200, nil)
	source := latestReviewTask(t, api, "evidence", reviewMediaID(url), "image")
	api.call("admin", "POST", "/admin/moderation/"+source.ID+"/decision", map[string]string{"decision": "reject", "reason": "源图违规"}, 200, nil)
	user, _ := get[User](t.Context(), api.App.DB, "users", "player", false)
	current, _ := get[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", reference.ID, false)
	if user.Avatar != rejectedImageURL || current.Status != "rejected" {
		t.Fatal("已通过引用没有随源图撤销")
	}
	url = api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 200)
	api.call("player", "PATCH", "/me/profile", map[string]string{"avatar": url}, 200, nil)
	if err := revokeReviewImage(t.Context(), api.App.DB, reviewMediaID(url)); err != nil {
		t.Fatal(err)
	}
	drainReviewAI(t, api)
	user, _ = get[User](t.Context(), api.App.DB, "users", "player", false)
	if user.Avatar != rejectedImageURL {
		t.Fatal("源图拒绝后的持久引用补偿没有恢复")
	}
}

// TestIntegrationPostReviewRevocationAndAuthorNotice 验证旧结构体不能恢复已撤销公开原图，订单拒绝通知送达实际提交打手并带有效入口。
func TestIntegrationPostReviewRevocationAndAuthorNotice(t *testing.T) {
	api := newTestAPI(t)
	url := api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 200)
	before, _ := get[PublicImage](t.Context(), api.App.DB, "evidence", reviewMediaID(url), false)
	task := latestReviewTask(t, api, "evidence", before.ID, "image")
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "原图不合规"}, 200, nil)
	if err := putRaw(t.Context(), api.App.DB, "evidence", before.ID, before); err != nil {
		t.Fatal(err)
	}
	after, _ := get[PublicImage](t.Context(), api.App.DB, "evidence", before.ID, false)
	if len(after.Data) != 0 || after.Moderation != "rejected" || after.Purpose != "avatar" {
		t.Fatal("旧结构体恢复了撤销内容或破坏归属")
	}
	order := api.newOrder()
	api.action("worker", order.ID, "accept", "", 200)
	api.action("worker", order.ID, "start", "", 200)
	api.action("worker", order.ID, "deliver", "需要人工隔离的交付说明", 200)
	delivery := latestReviewTask(t, api, "orders", order.ID, "delivery_note")
	if delivery.OwnerID != "worker" {
		t.Fatal("打手内容错误归属买家")
	}
	api.call("admin", "POST", "/admin/moderation/"+delivery.ID+"/decision", map[string]string{"decision": "reject", "reason": "交付说明不适合展示"}, 200, nil)
	var notices struct {
		Items []InboxMessage `json:"items"`
	}
	api.call("worker", "GET", "/messages", nil, 200, &notices)
	if len(notices.Items) != 1 || notices.Items[0].ActionPath != "/pages/orders/detail?id="+order.ID || notices.Items[0].ActionLabel == "" {
		t.Fatal("审核通知缺少真实业务入口或发错用户")
	}
}

// TestIntegrationPostReviewWorkerAndSubmitLimit 验证实际后台worker能消费任务，AI改后置后仍保留正式提交限流。
func TestIntegrationPostReviewWorkerAndSubmitLimit(t *testing.T) {
	api := newTestAPI(t)
	mockAllowAI(t, api)
	api.App.StartModerationWorker(t.Context())
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "由实际后台审核任务处理的简介"}, 200, nil)
	task := latestReviewTask(t, api, "users", "worker", "bio")
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		current, _ := get[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", task.ID, false)
		if current.Status == "allowed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	current, _ := get[ModerationTask](t.Context(), api.App.DB, "moderation_tasks", task.ID, false)
	if current.Status != "allowed" {
		t.Fatal("实际后台worker没有消费队列")
	}
	api.App.stopModerationWorker()
	api.App.Config.DevMode = false
	limitKey := api.App.redisKey("limit:moderation:worker:" + strconv.FormatInt(time.Now().Unix()/60, 10))
	if err := api.App.Redis.Set(t.Context(), limitKey, 90, 2*time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "不能突破原有提交限流的简介"}, 429, nil)
}

// TestIntegrationPostReviewDefaultAgreements 验证默认新版本条款、旧版本归档与运营者明确配置的保留。
func TestIntegrationPostReviewDefaultAgreements(t *testing.T) {
	for _, custom := range []bool{false, true} {
		name := "default"
		if custom {
			name = "operator-configured"
		}
		t.Run(name, func(t *testing.T) {
			api := newTestAPI(t)
			old := legacyBrandAgreements()
			if _, err := api.App.DB.Exec(t.Context(), "DELETE FROM settings WHERE id='default-post-review-agreements-v1'"); err != nil {
				t.Fatal(err)
			}
			if err := putRaw(t.Context(), api.App.DB, "settings", "agreements", old); err != nil {
				t.Fatal(err)
			}
			if custom {
				if err := audit(t.Context(), api.App.DB, "admin", "agreements.save", "agreements", "运营者确认的协议"); err != nil {
					t.Fatal(err)
				}
			}
			tx, err := api.App.DB.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(t.Context())
			if err = migratePostReviewDefaultAgreements(t.Context(), tx); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(t.Context()); err != nil {
				t.Fatal(err)
			}
			current, err := readAgreements(t.Context(), api.App.DB)
			if err != nil {
				t.Fatal(err)
			}
			if custom {
				if !sameAgreement(current.UserAgreement, old.UserAgreement) {
					t.Fatal("运营者协议被自动覆盖")
				}
			} else {
				if current.UserAgreement.Version != "2026-10-23.2" || strings.Contains(current.UserAgreement.Content, "审核服务故障时暂停提交") || !strings.Contains(current.UserAgreement.Content, "隔离保留") || !strings.Contains(current.PrivacyPolicy.Content, "人工或AI") {
					t.Fatal("当前条款与后置审核行为不符")
				}
				history, err := get[agreementDocument](t.Context(), api.App.DB, "settings", agreementHistoryID("user", old.UserAgreement.Version), false)
				if err != nil || !sameAgreement(history, old.UserAgreement) {
					t.Fatal("旧默认同意版本无法追溯")
				}
			}
		})
	}
}

// TestIntegrationPostReviewPlaceholderDimensions 验证占位图片不能沿用客户端伪造的尺寸撑开俱乐部图文页。
func TestIntegrationPostReviewPlaceholderDimensions(t *testing.T) {
	api := newTestAPI(t)
	var club Club
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"detail_blocks": []CommunityBlock{{Type: "image", ImageURL: rejectedImageURL, ImageWidth: 1, ImageHeight: 999999}}}, 200, &club)
	if len(club.DetailBlocks) != 1 || club.DetailBlocks[0].ImageWidth != 0 || club.DetailBlocks[0].ImageHeight != 0 {
		t.Fatal("占位图接受了客户端自填展示尺寸")
	}
}

// TestIntegrationPostReviewImageReferenceRetry 验证从业务图片字段点击重试，会一并唤醒故障后等待人工的源图任务。
func TestIntegrationPostReviewImageReferenceRetry(t *testing.T) {
	api := newTestAPI(t)
	url := api.uploadCommunityImage("player", "avatar", "", communityPNG(t, 256, 256), 200)
	api.call("player", "PATCH", "/me/profile", map[string]string{"avatar": url}, 200, nil)
	installMockAI(t, api, func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded })
	drainReviewAI(t, api)
	reference := latestReviewTask(t, api, "users", "player", "avatar")
	source := latestReviewTask(t, api, "evidence", reviewMediaID(url), "image")
	if reference.Status != "manual_pending" || source.Status != "manual_pending" {
		t.Fatal("故障没有保留源图及引用人工任务")
	}
	api.App.AIHTTPClient = &http.Client{Transport: paymentTransport(func(*http.Request) (*http.Response, error) {
		return aiMockResponse(`{"decision":"allow","reason":""}`), nil
	})}
	api.call("admin", "POST", "/admin/moderation/"+reference.ID+"/retry", map[string]any{}, 200, nil)
	drainReviewAI(t, api)
	reference = latestReviewTask(t, api, "users", "player", "avatar")
	source = latestReviewTask(t, api, "evidence", reviewMediaID(url), "image")
	if reference.Status != "allowed" || source.Status != "allowed" {
		t.Fatal("图片字段重试没有实际重新审核源图")
	}
}

// TestReviewRejectedBlockUsesPlaceholderRatio 验证图文原图撤销后同步采用真实320乘180占位比例，不沿用原长图高度。
func TestReviewRejectedBlockUsesPlaceholderRatio(t *testing.T) {
	block := map[string]any{"type": "image", "image_url": "/api/media/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "image_width": 640, "image_height": 4000}
	if !setReviewPath(block, []string{"image_url"}, rejectedImageURL) || block["image_width"] != 320 || block["image_height"] != 180 {
		t.Fatal("拒绝占位图沿用了原长图尺寸")
	}
}

// TestIntegrationPostReviewGameCategoryProjection 验证游戏分类被拒后，既有服务只屏蔽该来源分类且原商品与其他字段保留。
func TestIntegrationPostReviewGameCategoryProjection(t *testing.T) {
	api := newTestAPI(t)
	var game Game
	game, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	if err != nil {
		t.Fatal(err)
	}
	game.Categories = append(game.Categories, "需要隔离的分类")
	api.call("admin", "PATCH", "/admin/games/delta", game, 200, nil)
	var service Service
	api.call("worker", "POST", "/services", map[string]any{"game_id": "delta", "title": "应该保留的服务标题", "description": "这是应该保留的正常游戏任务说明", "category": "需要隔离的分类", "price_cents": 1000, "unit": "局", "cover_url": game.Covers[0]}, 200, &service)
	task := latestReviewTask(t, api, "games", "delta", "categories."+strconv.Itoa(len(game.Categories)-1))
	api.call("admin", "POST", "/admin/moderation/"+task.ID+"/decision", map[string]string{"decision": "reject", "reason": "该分类不合规"}, 200, nil)
	var shown Service
	api.call("", "GET", "/services/"+service.ID, nil, 200, &shown)
	if shown.Category != rejectedContentText || shown.Title != service.Title || shown.Description != service.Description {
		t.Fatal("服务分类漏屏蔽或误改其他可变字段")
	}
	stored, _ := get[Service](t.Context(), api.App.DB, "services", service.ID, false)
	if stored.Category != "需要隔离的分类" {
		t.Fatal("来源分类投影改写了商品原字段")
	}
}

// TestIntegrationPostReviewGameExternalImagesBlocked 验证管理员封面和图标不接受任意外链，旧外链记录不自动重写且不能用于新发布。
func TestIntegrationPostReviewGameExternalImagesBlocked(t *testing.T) {
	api := newTestAPI(t)
	game, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"cover", "icon", "gallery"} {
		copy := game
		copy.Covers = append([]string{}, game.Covers...)
		switch field {
		case "cover":
			copy.CoverURL = "https://example.com/unreviewed.jpg"
		case "icon":
			copy.IconURL = "https://example.com/unreviewed.png"
		case "gallery":
			copy.Covers = append(copy.Covers, "https://example.com/unreviewed.webp")
		}
		code, raw, e := api.request("admin", "PATCH", "/admin/games/delta", copy)
		if e != nil || code != 400 || !strings.Contains(string(raw), "先上传") {
			t.Fatalf("新增外链图绕过审查%s %d %s", field, code, raw)
		}
	}
	game.Covers = append(game.Covers, "https://example.com/legacy-stable-cover.jpg")
	if err = putRaw(t.Context(), api.App.DB, "games", game.ID, game); err != nil {
		t.Fatal(err)
	}
	stored, _ := get[Game](t.Context(), api.App.DB, "games", game.ID, false)
	if !contains(stored.Covers, "https://example.com/legacy-stable-cover.jpg") {
		t.Fatal("历史配置被自动改写")
	}
	api.call("worker", "POST", "/services", map[string]any{"game_id": "delta", "title": "不能引用外链的服务", "description": "这是正常的游戏服务发布说明内容", "category": game.Categories[0], "price_cents": 1000, "unit": "局", "cover_url": "https://example.com/legacy-stable-cover.jpg"}, 400, nil)
}
