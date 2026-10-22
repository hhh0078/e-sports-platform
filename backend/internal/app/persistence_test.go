// Package app 验证重启后身份、交易、证据和管理员配置仍来自持久化存储。
package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIntegrationRestartPreservesData 验证数据库迁移及种子不会覆盖真实使用数据。
func TestIntegrationRestartPreservesData(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	imageURL := api.uploadTestImage("player", o.ID)
	api.call("player", "POST", "/orders/"+o.ID+"/messages", map[string]string{"image_url": imageURL, "content": "重启前已记录的证据"}, 200, nil)
	api.call("admin", "PATCH", "/admin/settings", map[string]string{"site_name": "持久化开机电竞", "announcement": "重启不应覆盖这个公告"}, 200, nil)
	api.action("worker", o.ID, "accept", "", 200)
	a2, err := New(t.Context(), api.App.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer a2.Close()
	srv := httptest.NewServer(a2.Handler())
	defer srv.Close()
	other := &testAPI{t: t, Server: srv, App: a2, Tokens: api.Tokens}
	var saved Order
	other.call("player", "GET", "/orders/"+o.ID, nil, 200, &saved)
	if saved.Status != "accepted" || saved.AmountCents != 3800 {
		t.Fatalf("重启后订单丢失: %+v", saved)
	}
	var settings Settings
	other.call("admin", "GET", "/admin/settings", nil, 200, &settings)
	if settings.SiteName != "持久化开机电竞" {
		t.Fatal("重启覆盖了管理员配置")
	}
	var messages []Message
	other.call("clubowner", "GET", "/orders/"+o.ID+"/messages", nil, 200, &messages)
	found := false
	for _, m := range messages {
		if m.Kind == "image" && m.Content == "重启前已记录的证据" {
			found = true
			res, e := srv.Client().Get(srv.URL + m.ImageURL)
			if e != nil {
				t.Fatal(e)
			}
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatal("重启后必须重新签发可用图片链接")
			}
		}
	}
	if !found {
		t.Fatal("重启后图片聊天证据丢失")
	}
	other.call("player", "POST", "/auth/logout", map[string]any{}, 200, nil)
	other.call("player", "GET", "/me", nil, 401, nil)
}

// TestIntegrationServiceReauditAndSensitiveFields 验证服务修改重新审核，支付字段不能通过接口外泄。
func TestIntegrationServiceReauditAndSensitiveFields(t *testing.T) {
	api := newTestAPI(t)
	payload := map[string]any{"game_id": "delta", "title": "调整后的服务方案", "description": "修改价格后必须重新审核才允许继续向买家展示。", "category": "任务协作", "price_cents": 2300, "unit": "局", "cover_url": "/static/covers/official/delta-2.jpg", "labels": []CoverLabel{}, "tags": []string{}, "club_id": "shanhai"}
	var edited Service
	api.call("worker", "PATCH", "/services/delta-service-1", payload, 200, &edited)
	if edited.Status != "pending" {
		t.Fatal("修改服务必须重新审核")
	}
	api.call("", "GET", "/services/delta-service-1", nil, 404, nil)
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 409, nil)
	api.call("admin", "PATCH", "/admin/services/delta-service-1", map[string]string{"status": "active"}, 200, nil)
	o := api.newOrder()
	stored, err := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	stored.PayOpenID = "private-openid"
	stored.PaymentTransactionID = "private-transaction"
	stored.RefundID = "private-refund"
	if err = put(t.Context(), api.App.DB, "orders", o.ID, stored); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/orders/" + o.ID, "/orders", "/admin/orders"} {
		user := "player"
		if strings.HasPrefix(path, "/admin") {
			user = "admin"
		}
		status, raw, err := api.request(user, "GET", path, nil)
		if err != nil || status != 200 {
			t.Fatalf("读取失败 %d %v", status, err)
		}
		if strings.Contains(string(raw), "private-") {
			t.Fatalf("支付私密字段泄漏到 %s", path)
		}
	}
}
