// Package app 验证服务承接的真实身份门槛、预览防漂移、审核入队、事务回滚和历史资金隔离。
package app

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// conversionFixture 准备带真实格式微信核验记录的隔离测试管理员及130条演示服务。
func conversionFixture(t *testing.T) *testAPI {
	t.Helper()
	api := newTestAPI(t)
	if _, err := api.App.SeedShowcase(t.Context()); err != nil {
		t.Fatal(err)
	}
	identity := map[string]string{"openid": "conversion-fixture-openid", "phone_number": "13800138000", "phone_verified_at": now(), "phone_source": "wechat"}
	if err := put(t.Context(), api.App.DB, "identities", "admin", identity); err != nil {
		t.Fatal(err)
	}
	return api
}

// conversionProtectedSnapshot 只对不允许修改的表取摘要，检测交易、身份和资金被意外改写。
func conversionProtectedSnapshot(t *testing.T, api *testAPI) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range tables {
		if contains([]string{"services", "audit", "settings", "moderation_tasks", "moderation_targets"}, table) {
			continue
		}
		rows, err := list[map[string]any](t.Context(), api.App.DB, table, "ORDER BY id")
		if err != nil {
			t.Fatal(err)
		}
		out[table], err = conversionHash(rows)
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// TestConvertedShowcaseService 验证公开文案、独立ID、真实归属和清零统计，不复制虚构背书。
func TestConvertedShowcaseService(t *testing.T) {
	g := defaultGames()[0]
	source := Service{ID: "demo_service_001", GameID: g.ID, ProviderID: "demo_user_007", ProviderName: "旧打手·演示", ClubID: "demo_club_001", ClubName: "演示俱乐部", ClubVerified: true, Title: "三角洲·耐心陪练·演示", Description: "此为源码展示服务，不可付款下单。耐心陪练，提前沟通服务范围。所有图片文字均为可编辑展示示例。", Category: g.Categories[0], PriceCents: 900, Unit: "小时", CoverURL: g.Covers[0], Status: "active", CompletedCount: 8, Rating: 4.9, ProviderDepositCents: 999, ProviderClubDepositCents: 888, Labels: []CoverLabel{{ID: "demo_label_a", Text: "耐心陪练", Color: "#FFFFFF", Background: "transparent", Position: "bottom-left", FontSize: 16}, {ID: "demo_label_b", Text: "演示展示"}}, Tags: []string{"演示数据", "耐心沟通"}}
	owner := User{ID: "wx_real", Nickname: "真实接单人", Avatar: "/avatar.png"}
	got, err := convertedShowcaseService(source, owner, "2026-10-26T00:00:00Z", g)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(got.ID, "demo_") || got.ID == source.ID || got.ProviderID != owner.ID || got.ProviderName != owner.Nickname || got.ClubID != "" || got.ClubVerified || got.CompletedCount != 0 || got.Rating != 0 || got.ProviderDepositCents != 0 || got.ProviderClubDepositCents != 0 || len(got.Labels) != 1 || len(got.Tags) != 1 {
		t.Fatalf("转换隔离或文案清理异常：%+v", got)
	}
	raw, _ := json.Marshal(got)
	if hasShowcaseWording(string(raw)) {
		t.Fatalf("新服务仍有演示文案：%s", raw)
	}
	again, _ := convertedShowcaseService(source, owner, got.CreatedAt, g)
	if !reflect.DeepEqual(got, again) || source.ClubID == "" || source.Labels[0].ID != "demo_label_a" {
		t.Fatal("转换不稳定或修改了原服务")
	}
}

// TestIntegrationShowcaseConversionIsolation 验证130条转换、审核入队、幂等及重启不会复活旧服务。
func TestIntegrationShowcaseConversionIsolation(t *testing.T) {
	api := conversionFixture(t)
	protected := conversionProtectedSnapshot(t, api)
	var original []map[string]any
	original, err := list[map[string]any](t.Context(), api.App.DB, "services", "ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := api.App.PlanShowcaseServices(t.Context(), "admin")
	if err != nil || len(plan.Items) != 130 {
		t.Fatalf("预览失败：%v", err)
	}
	counts, err := api.App.ApplyShowcaseServices(t.Context(), plan)
	if err != nil || counts["created"] != 130 || counts["paused"] != 130 || counts["pending_review_fields"] != 650 {
		t.Fatalf("转换或审核入队失败：%+v %v", counts, err)
	}
	verified, err := api.App.VerifyShowcaseServices(t.Context())
	if err != nil || verified["live_ready"] != 130 || verified["old_paused"] != 130 {
		t.Fatalf("非扣款预检失败：%+v %v", verified, err)
	}
	for _, old := range original {
		id := reviewString(old, "id")
		got, err := get[map[string]any](t.Context(), api.App.DB, "services", id, false)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(id, "demo_") {
			old["status"] = "paused"
		}
		if !reflect.DeepEqual(old, got) {
			t.Fatalf("旧服务除下架外发生变更：%s", id)
		}
	}
	if !reflect.DeepEqual(protected, conversionProtectedSnapshot(t, api)) {
		t.Fatal("转换修改了历史交易、身份或资金表")
	}
	for _, source := range plan.Items {
		var tasks int
		if err = api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM moderation_tasks WHERE data->>'target_id'=$1 AND data->>'owner_id'='admin' AND data->>'status'='manual_pending'", source.Service.ID).Scan(&tasks); err != nil || tasks != 5 {
			t.Fatalf("服务未完整进入标准人工审核：%d %v", tasks, err)
		}
	}
	repeated, err := api.App.ApplyShowcaseServices(t.Context(), plan)
	if err != nil || repeated["already_applied"] != 130 || repeated["created"] != 0 {
		t.Fatalf("转换不幂等：%+v %v", repeated, err)
	}
	if _, err = api.App.SeedShowcase(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = api.App.seed(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err = api.App.VerifyShowcaseServices(t.Context()); err != nil {
		t.Fatalf("重复种子或重启复活了旧服务：%v", err)
	}
	api.call("", "GET", "/services/demo_service_001", nil, 404, nil)
	api.call("", "GET", "/services/"+plan.Items[0].Service.ID, nil, 200, nil)
	api.call("admin", "POST", "/orders/demo_order_001/pay", map[string]any{}, 400, nil)
}

// TestIntegrationShowcaseConversionRejectsDrift 验证最后一条源数据漂移也会回滚此前所有新服务和审核任务。
func TestIntegrationShowcaseConversionRejectsDrift(t *testing.T) {
	api := conversionFixture(t)
	if _, err := api.App.PlanShowcaseServices(t.Context(), "demo_user_007"); err == nil {
		t.Fatal("未拒绝虚构接单人")
	}
	plan, err := api.App.PlanShowcaseServices(t.Context(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	changed := plan
	changed.Items = append([]ServiceConversionItem{}, plan.Items...)
	changed.Items[0].Service.PriceCents++
	if _, err = api.App.ApplyShowcaseServices(t.Context(), changed); err == nil {
		t.Fatal("未拒绝预览篡改")
	}
	last := plan.Items[len(plan.Items)-1].SourceID
	raw, err := get[map[string]any](t.Context(), api.App.DB, "services", last, false)
	if err != nil {
		t.Fatal(err)
	}
	raw["description"] = reviewString(raw, "description") + "资料已修改。"
	if err = put(t.Context(), api.App.DB, "services", last, raw); err != nil {
		t.Fatal(err)
	}
	var tasksBefore, tasksAfter, live int
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM moderation_tasks").Scan(&tasksBefore)
	if _, err = api.App.ApplyShowcaseServices(t.Context(), plan); err == nil {
		t.Fatal("未拒绝源数据漂移")
	}
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM moderation_tasks").Scan(&tasksAfter)
	api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM services WHERE id=ANY($1)", []string{plan.Items[0].Service.ID, plan.Items[129].Service.ID}).Scan(&live)
	first, err := get[Service](t.Context(), api.App.DB, "services", plan.Items[0].SourceID, false)
	if err != nil || live != 0 || tasksBefore != tasksAfter || first.Status != "active" {
		t.Fatal("失败转换未完整回滚")
	}
}
