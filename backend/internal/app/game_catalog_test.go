// Package app 验证扩展游戏目录、官方素材升级、演示数据幂等和俱乐部游戏范围隔离。
package app

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestIntegrationExpandedGameCatalog 验证十三款游戏均能配置、搜索、发布、审核和创建订单。
func TestIntegrationExpandedGameCatalog(t *testing.T) {
	api := newTestAPI(t)
	var games []Game
	api.call("", "GET", "/games", nil, 200, &games)
	expected := []string{"honor", "peace", "delta", "tft", "valorant", "lol", "valorantm", "cfm", "arena", "codm", "identityv", "narakam", "eggy"}
	if len(games) != len(expected) {
		t.Fatalf("游戏数错误：%d", len(games))
	}
	for i, g := range games {
		if g.ID != expected[i] || g.IconURL != "/static/covers/official/"+g.ID+"-icon.jpg" {
			t.Fatalf("默认顺序或图标错误：%+v", g)
		}
		t.Run(g.ID, func(t *testing.T) {
			coverCount := 2
			if g.ID == "delta" || g.ID == "honor" {
				coverCount = 4
			}
			if len(g.Covers) != coverCount {
				t.Fatalf("%s 默认封面数量期望 %d，实际 %d", g.ID, coverCount, len(g.Covers))
			}
			var listed []Service
			api.call("", "GET", "/services?game_id="+g.ID, nil, 200, &listed)
			want := 2
			if g.ID == "delta" {
				want = 8
			} else if g.ID == "honor" {
				want = 4
			}
			if len(listed) != want {
				t.Fatalf("%s 示例数量期望 %d，实际 %d", g.ID, want, len(listed))
			}
			for _, s := range listed {
				if s.CompletedCount != 0 || s.Rating != 0 || !contains(g.Covers, s.CoverURL) {
					t.Fatalf("示例不得虚构成交或使用配置外封面：%+v", s)
				}
				if len(s.Labels) != 1 || s.Labels[0].ID != "title" || s.Labels[0].Color != "#07140D" || s.Labels[0].Background != g.PrimaryColor || s.Labels[0].Position != "top-left" || s.Labels[0].FontSize != 22 {
					t.Fatalf("新示例应使用单条清晰角标，旧服务标签仍由幂等插入保留：%+v", s.Labels)
				}
			}
			api.call("", "GET", "/services?game_id="+g.ID+"&q="+url.QueryEscape(g.Name), nil, 200, &listed)
			if len(listed) != want {
				t.Fatalf("按完整游戏名搜索漏失：%s", g.Name)
			}
			g.Subtitle = "管理员配置的游戏介绍"
			api.call("worker", "PATCH", "/admin/games/"+g.ID, g, 403, nil)
			api.call("admin", "PATCH", "/admin/games/"+g.ID, g, 200, nil)
			labels := []CoverLabel{{ID: "custom", Text: "一起组队", Color: "#AABBCC", Background: "#112233", Position: "bottom-right", FontSize: 18}}
			payload := map[string]any{"game_id": g.ID, "title": "独立封面文字的测试服务", "description": "双方确认范围后共同组队，结束后进行本局复盘。", "category": g.Categories[0], "price_cents": 1200, "unit": "小时", "cover_url": g.Covers[1], "labels": labels, "tags": []string{"耐心"}, "club_id": "shanhai"}
			var s Service
			api.call("worker", "POST", "/services", payload, 200, &s)
			if !reflect.DeepEqual(s.Labels, labels) || s.CoverURL != g.Covers[1] || s.Status != "active" {
				t.Fatalf("图片与文字必须独立保留：%+v", s)
			}
			api.call("", "GET", "/services/"+s.ID, nil, 200, nil)
			api.call("admin", "PATCH", "/admin/services/"+s.ID, map[string]string{"status": "active"}, 200, nil)
			var o Order
			api.call("player", "POST", "/orders", map[string]any{"service_id": s.ID, "quantity": 2}, 200, &o)
			if o.GameID != g.ID || o.AmountCents != 2400 || o.ClubID != "shanhai" {
				t.Fatalf("扩展游戏下单信息错误：%+v", o)
			}
		})
	}
	api.call("", "GET", "/services?q="+strings.Repeat("x", 101), nil, 400, nil)
}

// TestIntegrationOfficialCoverMigration 验证仅迁移识别到的旧封面且重复启动不覆盖管理员和用户修改。
func TestIntegrationOfficialCoverMigration(t *testing.T) {
	api := newTestAPI(t)
	o := api.newOrder()
	storedOrder, err := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	g, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
	if err != nil {
		t.Fatal(err)
	}
	g.Name, g.PrimaryColor, g.SortOrder, g.Enabled = "自定义三角洲", "#123456", 73, false
	g.IconURL = "https://example.com/custom-icon.png"
	g.CoverURL = "/static/covers/delta-2.jpg"
	g.Covers = []string{"https://example.com/custom-cover.png", "/static/covers/delta-2.jpg", "/static/covers/delta-3.jpg"}
	g.Categories = []string{"自定义服务"}
	if err = put(t.Context(), api.App.DB, "games", g.ID, g); err != nil {
		t.Fatal(err)
	}
	s, err := get[Service](t.Context(), api.App.DB, "services", "delta-service-1", false)
	if err != nil {
		t.Fatal(err)
	}
	s.Title, s.Description, s.Status, s.PriceCents = "用户自己的标题", "用户已经自定义了服务内容与价格。", "paused", 6789
	s.CoverURL = "/static/covers/delta-1.jpg"
	s.Labels = []CoverLabel{{ID: "mine", Text: "自定义文字", Color: "#ABCDEF", Background: "#012345", Position: "top-right", FontSize: 23}}
	if err = put(t.Context(), api.App.DB, "services", s.ID, s); err != nil {
		t.Fatal(err)
	}
	custom := s
	custom.ID, custom.CoverURL = "custom-service", "https://example.com/user-cover.jpg"
	if err = put(t.Context(), api.App.DB, "services", custom.ID, custom); err != nil {
		t.Fatal(err)
	}
	legacyCustom := s
	legacyCustom.ID, legacyCustom.CoverURL = "custom-third-cover", "/static/covers/delta-3.jpg"
	if err = put(t.Context(), api.App.DB, "services", legacyCustom.ID, legacyCustom); err != nil {
		t.Fatal(err)
	}
	if _, err = api.App.DB.Exec(t.Context(), "DELETE FROM settings WHERE id='catalog-official-v1'"); err != nil {
		t.Fatal(err)
	}
	if err = api.App.seed(t.Context()); err != nil {
		t.Fatal(err)
	}
	g.CoverURL = "/static/covers/official/delta-2.jpg"
	g.Covers[1] = g.CoverURL
	s.CoverURL = "/static/covers/official/delta-1.jpg"
	gotGame, _ := get[Game](t.Context(), api.App.DB, "games", g.ID, false)
	gotService, _ := get[Service](t.Context(), api.App.DB, "services", s.ID, false)
	gotCustom, _ := get[Service](t.Context(), api.App.DB, "services", custom.ID, false)
	gotLegacyCustom, _ := get[Service](t.Context(), api.App.DB, "services", legacyCustom.ID, false)
	gotOrder, _ := get[Order](t.Context(), api.App.DB, "orders", o.ID, false)
	if !reflect.DeepEqual(gotGame, g) || !reflect.DeepEqual(gotService, s) || !reflect.DeepEqual(gotCustom, custom) || !reflect.DeepEqual(gotLegacyCustom, legacyCustom) || !reflect.DeepEqual(gotOrder, storedOrder) {
		t.Fatal("迁移改变了非默认图片字段、用户文字或订单快照")
	}
	// 模拟迁移后管理员再次选择旧素材并清空图标，之后启动不应再替换。
	g.CoverURL, g.IconURL = "/static/covers/delta-1.jpg", ""
	if err = put(t.Context(), api.App.DB, "games", g.ID, g); err != nil {
		t.Fatal(err)
	}
	if err = api.App.seed(t.Context()); err != nil {
		t.Fatal(err)
	}
	gotGame, _ = get[Game](t.Context(), api.App.DB, "games", g.ID, false)
	if !reflect.DeepEqual(gotGame, g) {
		t.Fatal("重复启动覆盖了迁移后的管理员配置")
	}
}

// TestIntegrationDemoSeedPreservesClubScope 验证扩展目录不擅自扩展旧俱乐部支持游戏，也不重复插入示例。
func TestIntegrationDemoSeedPreservesClubScope(t *testing.T) {
	api := newTestAPI(t)
	club, err := get[Club](t.Context(), api.App.DB, "clubs", "shanhai", false)
	if err != nil {
		t.Fatal(err)
	}
	club.GameIDs = []string{"delta"}
	if err = put(t.Context(), api.App.DB, "clubs", club.ID, club); err != nil {
		t.Fatal(err)
	}
	if _, err = api.App.DB.Exec(t.Context(), "DELETE FROM services WHERE data->>'game_id'='lol'"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = api.App.seed(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	var services []Service
	api.call("", "GET", "/services?game_id=lol", nil, 200, &services)
	if len(services) != 2 {
		t.Fatalf("幂等补充数量错误：%d", len(services))
	}
	for _, s := range services {
		if s.ClubID != "" || s.ClubVerified || s.ClubName != "" {
			t.Fatal("新游戏示例不能冒用旧俱乐部不支持的游戏认证")
		}
	}
	current, _ := get[Club](t.Context(), api.App.DB, "clubs", club.ID, false)
	if !reflect.DeepEqual(current, club) {
		t.Fatal("补充示例不应覆盖旧俱乐部")
	}
}

// TestIntegrationClubScopeAndSingleConnection 验证单连接池创建十三游戏俱乐部并隔离移除游戏后的新订单。
func TestIntegrationClubScopeAndSingleConnection(t *testing.T) {
	database, err := url.Parse(LoadConfig().DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	params := database.Query()
	params.Set("pool_max_conns", "1")
	database.RawQuery = params.Encode()
	t.Setenv("DATABASE_URL", database.String())
	api := newTestAPI(t)
	api.Server.Client().Timeout = 2 * time.Second
	ids := []string{}
	for _, g := range defaultGames() {
		ids = append(ids, g.ID)
	}
	var created Club
	api.call("player", "POST", "/clubs", map[string]any{"name": "全游戏俱乐部", "description": "可以在单连接数据库中创建支持全部十三款游戏的俱乐部。", "game_ids": ids}, 200, &created)
	if len(created.GameIDs) != 13 {
		t.Fatal("俱乐部应可选择全部十三款游戏")
	}
	old := api.newOrder()
	api.call("clubowner", "PATCH", "/clubs/shanhai", map[string]any{"name": "山海电竞", "description": "调整俱乐部支持的游戏范围，只保留王者荣耀服务。", "game_ids": []string{"honor"}}, 200, nil)
	api.call("admin", "PATCH", "/admin/clubs/shanhai", map[string]any{"status": "active", "verified": true}, 200, nil)
	var service Service
	api.call("", "GET", "/services/delta-service-1", nil, 200, &service)
	if service.ClubVerified || service.ClubName != "" {
		t.Fatal("组织已移除的游戏不能继续展示其认证")
	}
	var current Order
	api.call("player", "POST", "/orders", map[string]any{"service_id": "delta-service-1", "quantity": 1}, 200, &current)
	if current.ClubID != "" {
		t.Fatal("移除游戏后的新订单不得再赋予该俱乐部权限")
	}
	api.call("clubowner", "GET", "/orders/"+current.ID, nil, 403, nil)
	api.call("clubowner", "GET", "/orders/"+old.ID, nil, 200, nil)
}

// TestGameIconValidation 验证官方子目录可配置，路径穿越和不安全图标协议不能进入游戏资料。
func TestGameIconValidation(t *testing.T) {
	g := defaultGames()[0]
	if err := validateGame(&g); err != nil {
		t.Fatal(err)
	}
	for _, icon := range []string{"/static/covers/official/../secret.png", "javascript:alert(1)", "http://example.com/icon.png"} {
		g.IconURL = icon
		if validateGame(&g) == nil {
			t.Fatalf("不安全图标被接受：%s", icon)
		}
	}
}

// TestIntegrationProductionCatalogHasNoDemo 验证非开发环境只建立游戏目录，不创建用户、组织或虚构服务。
func TestIntegrationProductionCatalogHasNoDemo(t *testing.T) {
	api := newTestAPI(t)
	if _, err := api.App.DB.Exec(t.Context(), "TRUNCATE users, services, clubs, members, games, settings"); err != nil {
		t.Fatal(err)
	}
	api.App.Config.DevMode = false
	if err := api.App.seed(t.Context()); err != nil {
		t.Fatal(err)
	}
	var games []Game
	api.call("", "GET", "/games", nil, 200, &games)
	if len(games) != 13 {
		t.Fatal("非开发环境也应具备完整可配置游戏目录")
	}
	for _, table := range []string{"users", "services", "clubs", "members"} {
		var n int
		if err := api.App.DB.QueryRow(t.Context(), "SELECT count(*) FROM "+validTable(table)).Scan(&n); err != nil || n != 0 {
			t.Fatalf("非开发环境不应创建 %s 演示记录：%d %v", table, n, err)
		}
	}
}

// TestIntegrationDefaultPresentationMigration 验证默认排序主题可升级，管理员审计或自定义数值均优先保留。
func TestIntegrationDefaultPresentationMigration(t *testing.T) {
	api := newTestAPI(t)
	legacyOrder := map[string]int{"delta": 0, "honor": 1, "peace": 2, "arena": 3, "valorant": 4, "cfm": 5, "tft": 6, "lol": 7, "valorantm": 8, "codm": 9, "narakam": 10, "identityv": 11, "eggy": 12}
	for id, order := range legacyOrder {
		g, err := get[Game](t.Context(), api.App.DB, "games", id, false)
		if err != nil {
			t.Fatal(err)
		}
		g.SortOrder = order
		if id == "delta" {
			g.PrimaryColor, g.BgColor, g.SurfaceColor = "#00E59B", "#080F0D", "#14221D"
		}
		if id == "peace" {
			g.SortOrder = 93
		}
		if err = put(t.Context(), api.App.DB, "games", id, g); err != nil {
			t.Fatal(err)
		}
	}
	if err := audit(t.Context(), api.App.DB, "admin", "game.save", "honor", "管理员保存过资料，不能推断未改排序"); err != nil {
		t.Fatal(err)
	}
	if _, err := api.App.DB.Exec(t.Context(), "DELETE FROM settings WHERE id='catalog-presentation-v1'"); err != nil {
		t.Fatal(err)
	}
	if err := api.App.seed(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, d := range defaultGames() {
		got, err := get[Game](t.Context(), api.App.DB, "games", d.ID, false)
		if err != nil {
			t.Fatal(err)
		}
		want := d.SortOrder
		if d.ID == "honor" {
			want = 1
		} else if d.ID == "peace" {
			want = 93
		}
		if got.SortOrder != want {
			t.Fatalf("%s 排序期望 %d，实际 %d", d.ID, want, got.SortOrder)
		}
		if d.ID == "delta" && (got.PrimaryColor != "#00E59B" || got.BgColor != "#0B0F0E" || got.SurfaceColor != "#101614") {
			t.Fatal("已证明未改的旧三角洲默认主题应升级")
		}
	}
	for _, scenario := range []struct {
		name, primary string
		audited       bool
	}{{"不同旧色不可猜测", "#71e2ab", false}, {"存在管理员审计即保留", "#00E59B", true}} {
		t.Run(scenario.name, func(t *testing.T) {
			g, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
			if err != nil {
				t.Fatal(err)
			}
			g.PrimaryColor, g.BgColor, g.SurfaceColor, g.SortOrder = scenario.primary, "#080F0D", "#14221D", 81
			if err = put(t.Context(), api.App.DB, "games", g.ID, g); err != nil {
				t.Fatal(err)
			}
			if scenario.audited {
				if err = audit(t.Context(), api.App.DB, "admin", "game.save", "delta", ""); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = api.App.DB.Exec(t.Context(), "DELETE FROM settings WHERE id='catalog-presentation-v1'"); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if err = api.App.seed(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			got, err := get[Game](t.Context(), api.App.DB, "games", "delta", false)
			if err != nil || !reflect.DeepEqual(got, g) {
				t.Fatalf("不得覆盖自定义主题或排序：%+v %v", got, err)
			}
		})
	}
}
