// Package app 初始化游戏主题及开发演示数据，生产环境不会创建测试管理员。
package app

import (
	"context"
	"encoding/json"
	"strconv"
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

// seed 填充游戏默认目录和本地多角色验收场景，已存在记录只执行明确的一次性图片迁移。
func (a *App) seed(ctx context.Context) error {
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(8173002)"); err != nil {
		return err
	}
	defaults := defaultGames()
	for _, g := range defaults {
		if err = seedInsert(ctx, tx, "games", g.ID, g); err != nil {
			return err
		}
	}
	if err = migrateOfficialCovers(ctx, tx, defaults); err != nil {
		return err
	}
	if err = migrateDefaultPresentation(ctx, tx, defaults); err != nil {
		return err
	}
	if err = seedInsert(ctx, tx, "settings", "platform", Settings{SiteName: "集结", Announcement: "找队友，也成为队友 · 下单前请与打手确认服务范围"}); err != nil {
		return err
	}
	if a.Config.DevMode {
		users := []User{{ID: "player", Nickname: "云间旅人", Bio: "喜欢和靠谱的队友一起玩游戏", Role: "user", Status: "active", ProviderEnabled: true}, {ID: "worker", Nickname: "北辰", Bio: "认真陪练，耐心沟通。本地示例打手，擅长战术配合与任务协作。", Role: "user", Status: "active", ProviderEnabled: true}, {ID: "clubowner", Nickname: "山海主理人", Bio: "山海电竞俱乐部负责人，帮助每位队友找到适合自己的舞台。", Role: "user", Status: "active", ProviderEnabled: true}, {ID: "admin", Nickname: "集结管理员", Bio: "平台运营与纠纷处理", Role: "admin", Status: "active", ProviderEnabled: true}, {ID: "outsider", Nickname: "自由玩家", Bio: "无挂靠也能接单，自由组队一起玩。", Role: "user", Status: "active", ProviderEnabled: true}}
		for _, u := range users {
			u.CreatedAt = now()
			if err = seedInsert(ctx, tx, "users", u.ID, u); err != nil {
				return err
			}
		}
		ids := make([]string, 0, len(defaults))
		for _, g := range defaults {
			ids = append(ids, g.ID)
		}
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
		club, err := get[Club](ctx, tx, "clubs", "shanhai", false)
		if err != nil {
			return err
		}
		member, err := get[ClubMember](ctx, tx, "members", "shanhai:worker", false)
		if err != nil {
			return err
		}
		for _, d := range defaults {
			g, err := get[Game](ctx, tx, "games", d.ID, false)
			if err != nil {
				return err
			}
			if !g.Enabled || len(g.Covers) == 0 {
				continue
			}
			for j, spec := range demoServices(g.ID) {
				if spec.Category >= len(g.Categories) {
					continue
				}
				// 管理员自定义分类后不再补充可能与新分类语义不符的演示商品。
				category := d.Categories[spec.Category]
				if !contains(g.Categories, category) {
					continue
				}
				title := spec.Title
				if title == "" {
					title = category + " · 默契队友"
				}
				s := Service{ID: g.ID + "-service-" + strconv.Itoa(j+1), GameID: g.ID, ProviderID: "worker", ProviderName: "北辰", Title: title, Description: "下单前请先说明游戏区服、希望完成的目标与可预约时间。服务包含组队沟通、战术配合及复盘指导；不需要提供账号密码，不使用任何外挂或违规工具。具体进度以双方确认的服务范围为准。", Category: category, PriceCents: spec.PriceCents, Unit: "小时", CoverURL: g.Covers[j%len(g.Covers)], Labels: []CoverLabel{{ID: "title", Text: spec.Label, Color: "#07140D", Background: g.PrimaryColor, Position: "top-left", FontSize: 22}}, Tags: []string{"本地示例", "耐心沟通"}, Status: "active", CreatedAt: now()}
				if club.Status == "active" && member.Status == "active" && contains(club.GameIDs, g.ID) {
					s.ClubID = club.ID
					s.ClubName = club.Name
					s.ClubVerified = club.Verified
				}
				if err = seedInsert(ctx, tx, "services", s.ID, s); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit(ctx)
}
