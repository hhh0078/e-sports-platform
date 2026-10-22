// Package app 验证系统通知投递、账号隔离、分页和已读删除不会触及订单沟通。
package app

import (
	"fmt"
	"strings"
	"testing"
)

// TestIntegrationInboxLifecycleAndPermissions 验证定向/广播、未读获取、本人幂等删除及越权隔离。
func TestIntegrationInboxLifecycleAndPermissions(t *testing.T) {
	api := newTestAPI(t)
	body := map[string]string{"recipient_id": "player", "title": "订单提醒", "body": "请查看本次平台服务通知"}
	api.call("", "GET", "/messages", nil, 401, nil)
	for _, user := range []string{"player", "worker", "clubowner", "outsider"} {
		api.call(user, "POST", "/admin/messages", body, 403, nil)
	}
	var sent struct {
		Count int `json:"recipient_count"`
	}
	api.call("admin", "POST", "/admin/messages", body, 200, &sent)
	if sent.Count != 1 {
		t.Fatal("定向通知接收人数错误")
	}
	var inbox struct {
		Items []InboxMessage `json:"items"`
		Total int            `json:"total"`
	}
	api.call("player", "GET", "/messages", nil, 200, &inbox)
	if inbox.Total != 1 || len(inbox.Items) != 1 || inbox.Items[0].Body != body["body"] {
		t.Fatal("未读通知未完整保存")
	}
	id := inbox.Items[0].ID
	api.call("outsider", "DELETE", "/messages/"+id, nil, 200, nil)
	api.call("player", "GET", "/messages", nil, 200, &inbox)
	if inbox.Total != 1 {
		t.Fatal("他人删除了通知")
	}
	api.call("player", "GET", "/messages", nil, 200, &inbox)
	if inbox.Total != 1 {
		t.Fatal("GET请求不应删除未读")
	}
	for i := 0; i < 2; i++ {
		api.call("player", "DELETE", "/messages/"+id, nil, 200, nil)
	}
	api.call("player", "GET", "/messages", nil, 200, &inbox)
	if inbox.Total != 0 {
		t.Fatal("本人已读删除失败")
	}
	api.call("admin", "POST", "/admin/messages", map[string]string{"title": "全体消息", "body": "发送给活跃的真实用户"}, 200, &sent)
	if sent.Count < 5 {
		t.Fatal("广播漏用户")
	}
	for _, user := range []string{"player", "worker", "clubowner", "outsider", "admin"} {
		api.call(user, "GET", "/messages", nil, 200, &inbox)
		if inbox.Total != 1 {
			t.Fatal("广播未独立投递")
		}
	}
	api.call("admin", "POST", "/admin/messages", map[string]string{"recipient_id": "missing-user", "title": "消息", "body": "正文"}, 404, nil)
	api.call("admin", "POST", "/admin/messages", map[string]string{"recipient_id": "player", "title": strings.Repeat("字", 61), "body": "正文"}, 400, nil)
	o := api.newOrder()
	var chats []Message
	api.call("player", "GET", "/orders/"+o.ID+"/messages", nil, 200, &chats)
	if len(chats) == 0 {
		t.Fatal("订单系统证据丢失")
	}
	api.call("player", "DELETE", "/messages/"+chats[0].ID, nil, 200, nil)
	api.call("player", "GET", "/orders/"+o.ID+"/messages", nil, 200, &chats)
	if len(chats) == 0 {
		t.Fatal("删除通知影响订单证据")
	}
}

// TestIntegrationInboxPaginationAndModerationDedup 验证超过一页及同一拒绝重复提交/已读后不会刷屏。
func TestIntegrationInboxPaginationAndModerationDedup(t *testing.T) {
	api := newTestAPI(t)
	for i := 0; i < 105; i++ {
		v := InboxMessage{ID: fmt.Sprintf("notice-%03d", i), UserID: "player", Title: fmt.Sprintf("系统消息%d", i), Body: "完整内容", CreatedAt: now()}
		if err := put(t.Context(), api.App.DB, "inbox", v.ID, v); err != nil {
			t.Fatal(err)
		}
	}
	var page struct {
		Items []InboxMessage `json:"items"`
		Total int            `json:"total"`
	}
	api.call("player", "GET", "/messages?limit=100", nil, 200, &page)
	if page.Total != 105 || len(page.Items) != 100 {
		t.Fatal("首页分页错误")
	}
	api.call("player", "GET", "/messages?limit=100&offset=100", nil, 200, &page)
	if len(page.Items) != 5 {
		t.Fatal("后续分页漏数据")
	}
	for i := 0; i < 2; i++ {
		api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "[审核拒绝测试] 需要修改的简介"}, 422, nil)
	}
	api.call("worker", "GET", "/messages", nil, 200, &page)
	if page.Total != 1 || page.Items[0].Title != "内容审核未通过" {
		t.Fatal("相同拒绝应只发一次系统消息")
	}
	api.call("worker", "DELETE", "/messages/"+page.Items[0].ID, nil, 200, nil)
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "[审核拒绝测试] 需要修改的简介"}, 422, nil)
	api.call("worker", "GET", "/messages", nil, 200, &page)
	if page.Total != 0 {
		t.Fatal("已读后相同拒绝重新刷屏")
	}
	api.call("worker", "PATCH", "/me/profile", map[string]string{"bio": "[审核故障测试] 保留资料"}, 503, nil)
}
