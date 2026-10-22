// Package app 验证接单身份恒定开启、历史迁移和列表兼容，同时保留冻结及手机号安全门槛。
package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestUserProviderReadCompatibility 确认旧关闭、缺失及空字段都按默认开启读取，不改变其它权限资料。
func TestUserProviderReadCompatibility(t *testing.T) {
	for _, field := range []string{"", `,"provider_enabled":false`, `,"provider_enabled":true`, `,"provider_enabled":null`} {
		var user User
		if err := json.Unmarshal([]byte(`{"id":"legacy","status":"frozen","phone_verified":false,"role":"user"`+field+`}`), &user); err != nil {
			t.Fatal(err)
		}
		if !user.ProviderEnabled || user.Status != "frozen" || user.PhoneVerified || user.Role != "user" {
			t.Fatal("接单默认值不应解冻账号、验证手机号或修改角色")
		}
	}
	var invalid User
	if json.Unmarshal([]byte(`{"provider_enabled":"false"}`), &invalid) == nil {
		t.Fatal("类型错误的数据不能被默认值掩盖")
	}
}

// TestProviderActionsIgnoreLegacySwitch 确认老用户的关闭字段不再阻止领取或接受订单。
func TestProviderActionsIgnoreLegacySwitch(t *testing.T) {
	user := User{ID: "provider", Nickname: "普通玩家", Status: "active"}
	for _, action := range []string{"claim", "accept"} {
		order := Order{BuyerID: "buyer", Status: "open", PaymentStatus: "paid"}
		if action == "accept" {
			order.ProviderID = user.ID
		}
		if err := applyAction(&order, user, action, ""); err != nil || order.Status != "accepted" {
			t.Fatal("有效用户不能被废弃的接单开关阻止履约")
		}
	}
}

// TestIntegrationProviderMigration 仅在独立测试schema迁移旧标记，验证幂等且保留全部其它用户字段。
func TestIntegrationProviderMigration(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.DB.Exec(t.Context(), `UPDATE users SET data=data||'{"provider_enabled":false}'::jsonb WHERE id='worker'; UPDATE users SET data=data-'provider_enabled' WHERE id='player'; UPDATE users SET data=data||'{"provider_enabled":false,"status":"frozen","phone_verified":false}'::jsonb WHERE id='outsider'`); err != nil {
		t.Fatal(err)
	}
	before := map[string]map[string]json.RawMessage{}
	for _, id := range []string{"worker", "player", "outsider"} {
		var raw []byte
		if err := api.App.DB.QueryRow(t.Context(), "SELECT data FROM users WHERE id=$1", id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var data map[string]json.RawMessage
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		delete(data, "provider_enabled")
		before[id] = data
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := migrate(t.Context(), api.App.DB); err != nil {
			t.Fatal(err)
		}
		for id, expected := range before {
			var raw []byte
			if err := api.App.DB.QueryRow(t.Context(), "SELECT data FROM users WHERE id=$1", id).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var data map[string]json.RawMessage
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			if string(data["provider_enabled"]) != "true" {
				t.Fatal("历史接单字段没有持久化为true")
			}
			delete(data, "provider_enabled")
			if !reflect.DeepEqual(data, expected) {
				t.Fatal("迁移改动了接单字段以外的用户资料")
			}
		}
	}
}

// TestIntegrationProviderDefaultAcrossRoutes 验证旧客户端不能关闭接单，原始旧标记不会隐藏合法服务或成员。
func TestIntegrationProviderDefaultAcrossRoutes(t *testing.T) {
	api := newTestAPI(t)
	var disabled int
	if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM users WHERE data->'provider_enabled' IS DISTINCT FROM 'true'::jsonb").Scan(&disabled); err != nil || disabled != 0 {
		t.Fatal("新种子用户没有全部默认开启接单")
	}
	code, raw, err := api.request("worker", "PATCH", "/me", map[string]any{"provider_enabled": false})
	if err != nil || code != 400 || !strings.Contains(string(raw), "默认开启") || !strings.Contains(string(raw), "不支持关闭") {
		t.Fatal("旧关闭请求必须得到清晰的不可关闭提示")
	}
	var stored bool
	if err := api.App.DB.QueryRow(t.Context(), "SELECT (data->>'provider_enabled')::boolean FROM users WHERE id='worker'").Scan(&stored); err != nil || !stored {
		t.Fatal("关闭请求改变了真实数据库的默认接单身份")
	}
	// 直接保留旧原始数据，证明读取和SQL列表均不依赖迁移已先行完成。
	if _, err := api.App.DB.Exec(t.Context(), `UPDATE users SET data=data||'{"provider_enabled":false}'::jsonb WHERE id='worker'; UPDATE users SET data=data-'provider_enabled' WHERE id='outsider'`); err != nil {
		t.Fatal(err)
	}
	var me map[string]any
	api.call("worker", "GET", "/me", nil, 200, &me)
	if me["provider_enabled"] != true {
		t.Fatal("旧账号资料没有按默认接单返回")
	}
	var services []Service
	api.call("", "GET", "/services?game_id=delta", nil, 200, &services)
	if len(services) == 0 {
		t.Fatal("旧接单标记错误地隐藏了合法服务")
	}
	api.call("", "GET", "/services/delta-service-1", nil, 200, nil)
	var player struct {
		Profile  map[string]any `json:"profile"`
		Services []Service      `json:"services"`
	}
	api.call("", "GET", "/players/worker", nil, 200, &player)
	if player.Profile["provider_enabled"] != true || len(player.Services) == 0 {
		t.Fatal("公开主页仍依赖可关闭的接单标记")
	}
	var club struct {
		Club     Club             `json:"club"`
		Members  []map[string]any `json:"members"`
		Services []Service        `json:"services"`
	}
	api.call("", "GET", "/clubs/shanhai", nil, 200, &club)
	if club.Club.MemberCount == 0 || len(club.Members) == 0 || club.Members[0]["certified"] != true || len(club.Services) == 0 {
		t.Fatal("有效俱乐部成员或服务仍被旧接单标记隐藏")
	}
	var order Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1, "requirements": "按约定完成服务"}, 200, &order)
	api.call("player", "POST", "/orders/"+order.ID+"/pay", map[string]any{}, 200, nil)
	api.call("worker", "POST", "/orders/"+order.ID+"/actions", map[string]any{"action": "accept"}, 200, nil)
	game, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"game_id": game.ID, "title": "默认接单身份服务", "description": "按约定协助完成游戏目标并同步进度。", "category": game.Categories[0], "price_cents": 2500, "unit": "小时", "cover_url": game.Covers[0]}
	api.call("outsider", "POST", "/services", input, 200, nil)
	if _, err = api.App.DB.Exec(t.Context(), "DELETE FROM identities WHERE id='outsider'"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/services", "/orders", "/requests"} {
		api.call("outsider", "POST", path, map[string]any{}, 428, nil)
	}
	api.call("admin", "PATCH", "/admin/users/worker", map[string]any{"status": "frozen"}, 200, nil)
	api.call("worker", "GET", "/me", nil, 403, nil)
	api.call("worker", "POST", "/orders/"+order.ID+"/actions", map[string]any{"action": "start"}, 403, nil)
	api.call("", "GET", "/players/worker", nil, 404, nil)
	api.call("", "GET", "/services/delta-service-1", nil, 404, nil)
	api.call("", "GET", "/clubs/shanhai", nil, 200, &club)
	if club.Club.MemberCount != 0 || len(club.Members) != 0 || len(club.Services) != 0 {
		t.Fatal("默认开启不应把冻结用户重新展示为认证成员或可售服务")
	}
}
