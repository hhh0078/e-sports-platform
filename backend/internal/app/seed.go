// Package app 初始化游戏主题及开发演示数据，生产环境不会创建测试管理员。
package app

import (
	"context"
	"encoding/json"
)

// seedInsert 仅插入不存在的数据，重启绝不覆盖管理员配置和历史记录。
func seedInsert(ctx context.Context, q querier, table, id string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, "INSERT INTO "+validTable(table)+"(id,data) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", id, raw)
	return err
}

// seed 填充七款游戏和本地多角色验收场景。
func (a *App) seed(ctx context.Context) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173002)"); err != nil {
		return err
	}
	defs := []struct {
		ID, Name, Short, Subtitle, Primary, BG, Surface string
		Categories                                      []string
	}{
		{"delta", "三角洲行动", "三角洲", "安全撤离，每次都有好队友", "#00E59B", "#080F0D", "#14221D", []string{"护航撤离", "任务协作", "技术陪练", "娱乐组队"}},
		{"honor", "王者荣耀", "王者", "峡谷相逢，一起向胜利出发", "#E9BC71", "#15100B", "#272016", []string{"双排陪玩", "五排组队", "英雄教学", "技术陪练"}},
		{"peace", "和平精英", "和平", "落地有队友，决赛圈见", "#66CBE1", "#09151C", "#132C37", []string{"娱乐组队", "技术陪练", "战术教学", "任务协作"}},
		{"arena", "暗区突围", "暗区", "准备周全，满载而归", "#E0AA62", "#14120D", "#292419", []string{"护航撤离", "地图教学", "任务协作", "娱乐组队"}},
		{"valorant", "无畏契约", "无畏", "默契配合，打出漂亮回合", "#FF6672", "#1A0E14", "#301923", []string{"双排陪玩", "战术教学", "技术陪练", "娱乐组队"}},
		{"cfm", "穿越火线手游", "穿越火线", "热血开局，兄弟并肩", "#F28C51", "#1A110D", "#302017", []string{"技术陪练", "娱乐组队", "地图教学", "任务协作"}},
		{"tft", "金铲铲之战", "金铲铲", "棋逢队友，弈起上场", "#E6C76D", "#17160D", "#2B2919", []string{"双人作战", "阵容教学", "新手指导", "娱乐组队"}},
	}
	games := []Game{}
	for i, d := range defs {
		g := Game{ID: d.ID, Name: d.Name, ShortName: d.Short, Subtitle: d.Subtitle, PrimaryColor: d.Primary, BgColor: d.BG, SurfaceColor: d.Surface, TextColor: "#F3F5F3", CoverURL: "/static/covers/" + d.ID + "-1.jpg", Covers: []string{"/static/covers/" + d.ID + "-1.jpg", "/static/covers/" + d.ID + "-2.jpg"}, Categories: d.Categories, Enabled: true, SortOrder: i}
		games = append(games, g)
		if err = seedInsert(ctx, tx, "games", g.ID, g); err != nil {
			return err
		}
	}
	if err = seedInsert(ctx, tx, "settings", "platform", Settings{SiteName: "集结", Announcement: "找队友，也成为队友 · 下单前请与打手确认服务范围"}); err != nil {
		return err
	}
	if a.Config.DevMode {
		users := []User{{ID: "player", Nickname: "云间旅人", Bio: "喜欢和靠谱的队友一起玩游戏", Role: "user", Status: "active"}, {ID: "worker", Nickname: "北辰", Bio: "认真陪练，耐心沟通。熟悉七款热门游戏，擅长战术配合与任务协作。", Role: "user", Status: "active", ProviderEnabled: true}, {ID: "clubowner", Nickname: "山海主理人", Bio: "山海电竞俱乐部负责人，帮助每位队友找到适合自己的舞台。", Role: "user", Status: "active", ProviderEnabled: true}, {ID: "admin", Nickname: "集结管理员", Bio: "平台运营与纠纷处理", Role: "admin", Status: "active"}, {ID: "outsider", Nickname: "自由玩家", Bio: "无挂靠也能接单，自由组队一起玩。", Role: "user", Status: "active", ProviderEnabled: true}}
		for _, u := range users {
			u.CreatedAt = now()
			if err = seedInsert(ctx, tx, "users", u.ID, u); err != nil {
				return err
			}
		}
		ids := []string{"delta", "honor", "peace", "arena", "valorant", "cfm", "tft"}
		clubs := []Club{{ID: "shanhai", Name: "山海电竞", Description: "始于热爱，忠于默契。为队友提供技术陪练、战术教学与任务协作服务。", OwnerID: "clubowner", OwnerName: "山海主理人", GameIDs: ids, Status: "active", Verified: true, CreatedAt: now()}, {ID: "wild", Name: "原野玩家社", Description: "自由玩家的聚集地，一起寻找游戏乐趣，认识志同道合的新朋友。", OwnerID: "outsider", OwnerName: "自由玩家", GameIDs: []string{"delta", "peace", "tft"}, Status: "active", Verified: false, CreatedAt: now()}}
		for _, c := range clubs {
			if err = seedInsert(ctx, tx, "clubs", c.ID, c); err != nil {
				return err
			}
		}
		m := ClubMember{UserID: "worker", Nickname: "北辰", ClubID: "shanhai", Status: "active", CreatedAt: now()}
		if err = seedInsert(ctx, tx, "members", "shanhai:worker", m); err != nil {
			return err
		}
		for _, g := range games {
			for j := 0; j < 2; j++ {
				id := g.ID + "-service-" + string(rune('1'+j))
				title := g.Categories[j] + " · 默契队友"
				if g.ID == "delta" {
					title = []string{"安全撤离 · 全程护航", "任务协作 · 轻松探索"}[j]
				}
				s := Service{ID: id, GameID: g.ID, ProviderID: "worker", ProviderName: "北辰", ClubID: "shanhai", ClubName: "山海电竞", ClubVerified: true, Title: title, Description: "下单前请先说明游戏区服、希望完成的目标与可预约时间。服务包含组队沟通、战术配合及复盘指导；不需要提供账号密码，不使用任何外挂或违规工具。具体进度以双方确认的服务范围为准。", Category: g.Categories[j], PriceCents: int64(1900 + j*1000), Unit: "小时", CoverURL: g.Covers[j], Labels: []CoverLabel{{ID: "title", Text: []string{"默契组队", "耐心陪练"}[j], Color: "#FFFFFF", Background: "transparent", Position: "bottom-left", FontSize: 20}, {ID: "tag", Text: "安心服务", Color: g.PrimaryColor, Background: "#102019", Position: "top-left", FontSize: 12}}, Tags: []string{"俱乐部认证", "耐心沟通"}, Status: "active", CreatedAt: now()}
				if err = seedInsert(ctx, tx, "services", s.ID, s); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit(ctx)
}
